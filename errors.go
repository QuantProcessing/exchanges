package exchanges

import (
	"errors"
	"fmt"
)

var ErrRateLimited = errors.New("rate limited")

type ExchangeError struct {
	Exchange string
	Code     string
	Message  string
	Err      error
}

func NewExchangeError(exchange, code, message string, err error) *ExchangeError {
	return &ExchangeError{Exchange: exchange, Code: code, Message: message, Err: err}
}

func (e *ExchangeError) Error() string {
	if e == nil {
		return ""
	}
	if e.Code == "" {
		return fmt.Sprintf("%s: %s", e.Exchange, e.Message)
	}
	return fmt.Sprintf("%s %s: %s", e.Exchange, e.Code, e.Message)
}

func (e *ExchangeError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}
