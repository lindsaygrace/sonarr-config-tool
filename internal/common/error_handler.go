package common

import (
	"context"

	"github.com/getsentry/sentry-go"
)

// ErrorHandler handles an error.
type ErrorHandler interface {
	Handle(err error)
	HandleContext(ctx context.Context, err error)
}

// NoopErrorHandler is an error handler that discards every error.
type NoopErrorHandler struct{}

// Handle ignore
func (NoopErrorHandler) Handle(_ error) {}

// HandleContext ignore
func (NoopErrorHandler) HandleContext(_ context.Context, _ error) {}

// HandleError wraps ErrorHandler and Errors to perform additional tasks, such as sending an error to Sentry
// before sending it on to the error handler
func HandleError(errorHandler ErrorHandler, err error) {

	// send error to Sentry
	sentry.CaptureException(err)

	// forward error to handler for logging
	errorHandler.Handle(err)
}
