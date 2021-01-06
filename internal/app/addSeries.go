package app

import (
	"fmt"
	"github.com/lindsaygrace/go-sonarr-client"
	"github.com/lindsaygrace/sonarr-config-tool/internal/app/utils"
	"os"
	"path/filepath"
)

func addSeries(dir *os.File, tvPath string, client *sonarr.Sonarr, logger Logger, errorHandler ErrorHandler) error {

	var series *sonarr.Series

	fileInfo, err := dir.Stat()
	if err != nil {
		return err
	}

	id, err := utils.GetSeriesID(dir, client, logger)
	if err != nil {
		return err
	}

	series, err = client.GetSeriesFromTVDB(id)
	if err != nil {
		return err
	}

	logger.Debug(fmt.Sprintf("Found series %s", series.Title))

	series.Path = filepath.Join(tvPath, fileInfo.Name())
	logger.Debug(fmt.Sprintf("Setting series Path to %s", series.Path))

	logger.Debug("Setting series QualityProfileID to 1")
	series.QualityProfileID = 1

	errs := client.AddSeries(*series)
	for _, err := range errs {
		return err
	}

	return nil
}
