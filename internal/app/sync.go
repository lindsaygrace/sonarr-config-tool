package app

import (
	"fmt"
	"github.com/lindsaygrace/go-sonarr-client"
	"io/ioutil"
	"os"

	"path/filepath"
)

func sync(path string, tvPath string, client *sonarr.Sonarr, logger Logger, errorHandler ErrorHandler) error {

	filePath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	files, err := ioutil.ReadDir(filePath)
	if err != nil {
		return err
	}

	for _, f := range files {
		if f.IsDir() {
			logger.Info(fmt.Sprintf("Adding series %s", f.Name()))
			dir, err := os.Open(filepath.Join(filePath, f.Name()))
			if err != nil {
				handleError(errorHandler, err)
			} else {
				err := addSeries(dir, tvPath, client, logger, errorHandler)
				if err != nil {
					handleError(errorHandler, err)
				}
			}
		}
	}

	return nil
}
