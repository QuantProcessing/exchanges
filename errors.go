package exchanges

import "github.com/QuantProcessing/exchanges/internal/errs"

// ============================================================================
// Sentinel Errors — structured error handling for trading operations
// ============================================================================

var (
	ErrInsufficientBalance = errs.ErrInsufficientBalance
	ErrRateLimited         = errs.ErrRateLimited
	ErrInvalidPrecision    = errs.ErrInvalidPrecision
	ErrOrderNotFound       = errs.ErrOrderNotFound
	ErrSymbolNotFound      = errs.ErrSymbolNotFound
	ErrMinNotional         = errs.ErrMinNotional
	ErrMinQuantity         = errs.ErrMinQuantity
	ErrAuthFailed          = errs.ErrAuthFailed
	ErrNetworkTimeout      = errs.ErrNetworkTimeout
	ErrNotSupported        = errs.ErrNotSupported
)

// ExchangeError wraps an exchange-specific error with a sentinel cause.
// Use errors.Is(err, adapter.ErrInsufficientBalance) for structured handling.
type ExchangeError = errs.ExchangeError

// NewExchangeError creates a new ExchangeError.
func NewExchangeError(exchange, code, message string, sentinel error) *ExchangeError {
	return errs.NewExchangeError(exchange, code, message, sentinel)
}
