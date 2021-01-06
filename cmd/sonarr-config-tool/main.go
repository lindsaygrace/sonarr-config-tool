package main

import (
	"context"
	"fmt"
	"github.com/lindsaygrace/go-sonarr-client"
	"net/http"
	_ "net/http/pprof" // register pprof HTTP handlers #nosec
	"os"
	"os/signal"
	"syscall"
	"time"

	"contrib.go.opencensus.io/exporter/ocagent"
	"emperror.dev/emperror"
	"emperror.dev/errors"
	"emperror.dev/errors/match"
	logurhandler "emperror.dev/handler/logur"
	health "github.com/AppsFlyer/go-sundheit"
	healthhttp "github.com/AppsFlyer/go-sundheit/http"
	"github.com/cloudflare/tableflip"
	"github.com/oklog/run"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sagikazarmark/appkit/buildinfo"
	appkiterrors "github.com/sagikazarmark/appkit/errors"
	appkitrun "github.com/sagikazarmark/appkit/run"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/trace"
	"go.opencensus.io/zpages"
	"logur.dev/logur"

	"github.com/lindsaygrace/sonarr-config-tool/internal/app"
	"github.com/lindsaygrace/sonarr-config-tool/internal/common/commonadapter"
	"github.com/lindsaygrace/sonarr-config-tool/internal/platform/appkit"
	"github.com/lindsaygrace/sonarr-config-tool/internal/platform/gosundheit"
	"github.com/lindsaygrace/sonarr-config-tool/internal/platform/log"

	"github.com/getsentry/sentry-go"
)

// Provisioned by ldflags
// nolint: gochecknoglobals
var (
	version    string
	commitHash string
	buildDate  string
)

const (
	// appName is an identifier-like name used anywhere this app needs to be identified.
	//
	// It identifies the application itself, the actual instance needs to be identified via environment
	// and other details.
	appName = "sonarr-config-tool"

	// friendlyAppName is the visible name of the application.
	friendlyAppName = "Sonarr Config Tool"
)

func main() {
	v, p := viper.New(), pflag.NewFlagSet(friendlyAppName, pflag.ExitOnError)

	configure(v, p)

	p.String("config", "", "Configuration file")
	p.Bool("version", false, "Show version information")

	_ = p.Parse(os.Args[1:])

	if v, _ := p.GetBool("version"); v {
		fmt.Printf("%s version %s (%s) built on %s\n", friendlyAppName, version, commitHash, buildDate)

		os.Exit(0)
	}

	if c, _ := p.GetString("config"); c != "" {
		v.SetConfigFile(c)
	}

	err := v.ReadInConfig()
	_, configFileNotFound := err.(viper.ConfigFileNotFoundError)
	if !configFileNotFound {
		emperror.Panic(errors.Wrap(err, "failed to read configuration"))
	}

	var config configuration
	err = v.Unmarshal(&config)
	emperror.Panic(errors.Wrap(err, "failed to unmarshal configuration"))

	err = config.Process()
	emperror.Panic(errors.WithMessage(err, "failed to process configuration"))

	// Create logger (first thing after configuration loading)
	logger := log.NewLogger(config.Log)

	// Override the global standard library logger to make sure everything uses our logger
	log.SetStandardLogger(logger)

	if configFileNotFound {
		logger.Warn("configuration file not found")
	}

	err = config.Validate()
	if err != nil {
		logger.Error(err.Error())

		os.Exit(3)
	}

	// Configure error handler
	errorHandler := logurhandler.New(logger)
	defer emperror.HandleRecover(errorHandler)

	// Configure Sentry
	err = sentry.Init(sentry.ClientOptions{
		// Either set your DSN here or set the SENTRY_DSN environment variable.
		Dsn: config.Sentry.Dsn,
		// Either set environment and release here or set the SENTRY_ENVIRONMENT
		// and SENTRY_RELEASE environment variables.
		Environment: config.Sentry.Environment,
		Release:     commitHash,
		// Enable printing of SDK debug messages.
		// Useful when getting started or trying to figure something out.
		Debug: config.Sentry.Debug,
	})
	if err != nil {
		emperror.Panic(err)
	}

	// Flush buffered events before the program terminates.
	// Set the timeout to the maximum duration the program can afford to wait.
	defer sentry.Flush(2 * time.Second)

	defer sentry.Recover()

	buildInfo := buildinfo.New(version, commitHash, buildDate)

	logger.Info("starting application", buildInfo.Fields())

	telemetryRouter := http.NewServeMux()
	telemetryRouter.Handle("/buildinfo", buildinfo.HTTPHandler(buildInfo))

	// Register pprof endpoints
	telemetryRouter.Handle("/debug/pprof/", http.DefaultServeMux)

	// Configure health checker
	healthChecker := health.New()
	healthChecker.WithCheckListener(gosundheit.NewLogger(logur.WithField(logger, "component", "healthcheck")))
	{
		handler := healthhttp.HandleHealthJSON(healthChecker)
		telemetryRouter.Handle("/healthz", handler)

		// Kubernetes style health checks
		telemetryRouter.HandleFunc("/healthz/live", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("ok"))
		})
		telemetryRouter.Handle("/healthz/ready", handler)
	}

	zpages.Handle(telemetryRouter, "/debug")

	trace.ApplyConfig(config.Opencensus.Trace.Config())

	// Configure OpenCensus exporter
	if config.Opencensus.Exporter.Enabled {
		exporter, err := ocagent.NewExporter(append(
			config.Opencensus.Exporter.Options(),
			ocagent.WithServiceName(appName),
		)...)
		emperror.Panic(err)

		trace.RegisterExporter(exporter)
		view.RegisterExporter(exporter)
	}

	// Configure Prometheus exporter
	if config.Prometheus.Enabled {
		telemetryRouter.Handle("/metrics", promhttp.Handler())
	}

	// configure graceful restart
	upg, _ := tableflip.New(tableflip.Options{})

	// Do an upgrade on SIGHUP
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGHUP)
		for range ch {
			logger.Info("graceful reloading")

			_ = upg.Upgrade()
		}
	}()

	var group run.Group

	// Set up telemetry server
	{
		const name = "telemetry"
		logger := logur.WithField(logger, "server", name)

		logger.Info("listening on address", map[string]interface{}{"address": config.Telemetry.Addr})

		ln, err := upg.Fds.Listen("tcp", config.Telemetry.Addr)
		if err != nil {
			sentry.CaptureException(err)
		}
		emperror.Panic(err)

		server := &http.Server{
			Handler:  telemetryRouter,
			ErrorLog: log.NewErrorStandardLogger(logger),
		}
		defer server.Close()

		group.Add(
			func() error { return server.Serve(ln) },
			func(err error) { _ = server.Shutdown(context.Background()) },
		)
	}

	// Register stat views
	err = view.Register(
		// Health checks
		health.ViewCheckCountByNameAndStatus,
		health.ViewCheckStatusByName,
		health.ViewCheckExecutionTime,

		// HTTP
		ochttp.ServerRequestCountView,
		ochttp.ServerRequestBytesView,
		ochttp.ServerResponseBytesView,
		ochttp.ServerLatencyView,
		ochttp.ServerRequestCountByMethod,
		ochttp.ServerResponseCountByStatusCode,
	)
	emperror.Panic(errors.Wrap(err, "failed to register stat views"))

	// Set up app server
	{
		const name = "app"
		logger := logur.WithField(logger, "server", name)

		// In larger apps, this should be split up into smaller functions
		{
			logger := commonadapter.NewContextAwareLogger(logger, appkit.ContextExtractor)
			errorHandler := emperror.WithFilter(
				emperror.WithContextExtractor(errorHandler, appkit.ContextExtractor),
				appkiterrors.IsServiceError, // filter out service errors
			)

			sonarrClient, err := sonarr.New(config.Sonarr.APIURL, config.Sonarr.APIKey)
			if err != nil {
				logger.Error(err.Error())
			}

			logger.Info("Initialising app", map[string]interface{}{"path": config.App.Path})
			go func() {
				defer sentry.Flush(2 * time.Second)
				defer sentry.Recover()
				app.InitializeApp(config.App.Path, config.Sonarr.TvPath, sonarrClient, logger, errorHandler)
			}()

		}

	}

	// Setup signal handler
	group.Add(run.SignalHandler(context.Background(), syscall.SIGINT, syscall.SIGTERM))

	// Setup graceful restart
	group.Add(appkitrun.GracefulRestart(context.Background(), upg))

	err = group.Run()
	if err != nil {
		if match.As(&run.SignalError{}).MatchError(err) == false {
			sentry.CaptureException(err)
		}
	}
	emperror.WithFilter(errorHandler, match.As(&run.SignalError{}).MatchError).Handle(err)

}
