package strategy

import (
	"io"
	"log/slog"
)

var discardLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

func loggerOrDiscard(logger *slog.Logger) *slog.Logger {
	if logger == nil {
		return discardLogger
	}
	return logger
}
