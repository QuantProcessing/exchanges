package exchanges

// Logger is the logging interface used throughout the library.
// It is compatible with *zap.SugaredLogger out of the box.
//
// Usage:
//
//	// With zap:
//	logger := zap.NewProduction().Sugar()
//	adp, err := binance.NewAdapter(ctx, binance.Options{Logger: logger})
//
//	// With NopLogger (default when no logger is provided):
//	adp, err := binance.NewAdapter(ctx, binance.Options{})
type Logger interface {
	Debugw(msg string, keysAndValues ...any)
	Infow(msg string, keysAndValues ...any)
	Warnw(msg string, keysAndValues ...any)
	Errorw(msg string, keysAndValues ...any)
}

// NopLogger discards all log output. This is the default when no logger is provided.
var NopLogger Logger = nopLogger{}

type nopLogger struct{}

func (nopLogger) Debugw(string, ...any) {}
func (nopLogger) Infow(string, ...any)  {}
func (nopLogger) Warnw(string, ...any)  {}
func (nopLogger) Errorw(string, ...any) {}
