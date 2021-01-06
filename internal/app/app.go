package app

import "github.com/lindsaygrace/go-sonarr-client"

// InitializeApp initializes a new file watch and sync application.
func InitializeApp(
	path string,
	tvPath string,
	client *sonarr.Sonarr,
	logger Logger,
	errorHandler ErrorHandler, // nolint: interfacer
) {
	err := sync(path, tvPath, client, logger, errorHandler)
	if err != nil {
		handleError(errorHandler, err)
	}
}
