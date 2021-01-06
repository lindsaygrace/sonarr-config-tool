package main

import (
	"errors"
	"os"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/lindsaygrace/sonarr-config-tool/internal/platform/log"
	"github.com/lindsaygrace/sonarr-config-tool/internal/platform/opencensus"
)

// configuration holds any kind of configuration that comes from the outside world and
// is necessary for running the application.
type configuration struct {
	// Log configuration
	Log log.Config

	// Telemetry configuration
	Telemetry struct {
		// Telemetry HTTP server address
		Addr string
	}

	// OpenCensus configuration
	Opencensus struct {
		Exporter struct {
			Enabled bool

			opencensus.ExporterConfig `mapstructure:",squash"`
		}

		Trace opencensus.TraceConfig
	}

	// Prometheus configuration
	Prometheus struct {
		Enabled bool
	}

	// Sentry configuration
	Sentry struct {
		Dsn         string
		Environment string
		Release     string
		Debug       bool
	}

	// Sonarr configuration
	Sonarr struct {
		APIURL string
		APIKey string
		TvPath string
	}

	// App configuration
	App appConfig
}

// Process post-processes configuration after loading it.
func (configuration) Process() error {
	return nil
}

// Validate validates the configuration.
func (c configuration) Validate() error {
	if c.Telemetry.Addr == "" {
		return errors.New("telemetry http server address is required")
	}

	if err := c.App.Validate(); err != nil {
		return err
	}

	return nil
}

// appConfig represents the application related configuration.
type appConfig struct {
	// App file sync path
	// nolint: golint, stylecheck
	Path string
}

// Validate validates the configuration.
func (c appConfig) Validate() error {
	if c.Path == "" {
		return errors.New("path filepath is required")
	}

	return nil
}

// configure configures some defaults in the Viper instance.
func configure(v *viper.Viper, p *pflag.FlagSet) {
	// Viper settings
	v.AddConfigPath(".")
	v.AddConfigPath("$CONFIG_DIR/")

	// Environment variable settings
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AllowEmptyEnv(true)
	v.AutomaticEnv()

	// Global configuration
	v.SetDefault("shutdownTimeout", 15*time.Second)
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		v.SetDefault("no_color", true)
	}

	// Log configuration
	v.SetDefault("log.format", "json")
	v.SetDefault("log.level", "info")
	v.RegisterAlias("log.noColor", "no_color")

	// Telemetry configuration
	p.String("telemetry-addr", ":10000", "Telemetry HTTP server address")
	_ = v.BindPFlag("telemetry.addr", p.Lookup("telemetry-addr"))
	v.SetDefault("telemetry.addr", ":10000")

	// OpenCensus configuration
	v.SetDefault("opencensus.exporter.enabled", true)
	_ = v.BindEnv("opencensus.exporter.address")
	_ = v.BindEnv("opencensus.exporter.insecure")
	_ = v.BindEnv("opencensus.exporter.reconnectPeriod")
	v.SetDefault("opencensus.trace.sampling.sampler", "never")
	v.SetDefault("prometheus.enabled", true)

	// Sentry configuration
	p.Bool("sentry-debug", false, "Whether Sentry debug mode is enabled")
	_ = v.BindPFlag("sentry.debug", p.Lookup("sentry-debug"))
	v.SetDefault("sentry.debug", false)
	p.String("sentry-dsn", "", "Either set your DSN here or set the SENTRY_DSN environment variable")
	_ = v.BindPFlag("sentry.dsn", p.Lookup("sentry-dsn"))
	v.SetDefault("sentry.dsn", "")
	p.String("sentry-environment", "", "Either set environment here or set the SENTRY_ENVIRONMENT environment variable")
	_ = v.BindPFlag("sentry.environment", p.Lookup("sentry-environment"))
	v.SetDefault("sentry.environment", "")
	p.String("sentry-release", "", "Either set release here or set the SENTRY_RELEASE environment variable")
	_ = v.BindPFlag("sentry.release", p.Lookup("sentry-release"))
	v.SetDefault("sentry.release", "")

	// Sonarr configuration
	p.String("sonarr-apiurl", "", "Sonarr Api Url")
	_ = v.BindPFlag("sonarr.apiurl", p.Lookup("sonarr-apiurl"))
	p.String("sonarr-apikey", "", "Sonarr Api Key")
	_ = v.BindPFlag("sonarr.apikey", p.Lookup("sonarr-apikey"))
	p.String("sonarr-tvpath", "/tv shows", "Sonarr TV Shows directory")
	_ = v.BindPFlag("sonarr.tvpath", p.Lookup("sonarr-tvpath"))

	// App configuration
	p.String("path", ".", "Local media directory")
	_ = v.BindPFlag("app.path", p.Lookup("path"))

}
