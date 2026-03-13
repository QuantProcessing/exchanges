package exchanges

import (
	"errors"
	"fmt"
)

// ============================================================================
// Sentinel Errors — structured error handling for trading operations
// ============================================================================

var (
	ErrInsufficientBalance = errors.New("insufficient balance")
	ErrRateLimited         = errors.New("rate limited")
	ErrInvalidPrecision    = errors.New("invalid precision")
	ErrOrderNotFound       = errors.New("order not found")
	ErrSymbolNotFound      = errors.New("symbol not found")
	ErrMinNotional         = errors.New("below minimum notional")
	ErrMinQuantity         = errors.New("below minimum quantity")
	ErrAuthFailed          = errors.New("authentication failed")
	ErrNetworkTimeout      = errors.New("network timeout")
	ErrNotSupported        = errors.New("not supported")
)

// ExchangeError wraps an exchange-specific error with a sentinel cause.
// Use errors.Is(err, adapter.ErrInsufficientBalance) for structured handling.
type ExchangeError struct {
	Exchange string // Exchange name, e.g. "BINANCE"
	Code     string // Exchange-specific error code
	Message  string // Original error message from exchange
	Err      error  // Sentinel error for errors.Is matching
}

func (e *ExchangeError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("[%s] %s: %s", e.Exchange, e.Code, e.Message)
	}
	return fmt.Sprintf("[%s] %s", e.Exchange, e.Message)
}

func (e *ExchangeError) Unwrap() error {
	return e.Err
}

// NewExchangeError creates a new ExchangeError.
func NewExchangeError(exchange, code, message string, sentinel error) *ExchangeError {
	return &ExchangeError{
		Exchange: exchange,
		Code:     code,
		Message:  message,
		Err:      sentinel,
	}
}
