package strategy

import (
	"errors"
	"fmt"
	"time"
)

var ErrInvalidTimer = errors.New("invalid timer")

type Clock interface {
	Now() time.Time
}

type WallClock struct{}

func (WallClock) Now() time.Time {
	return time.Now()
}

type TimerEvent struct {
	Name      string
	Timestamp time.Time
}

func ValidateTimer(name string, interval time.Duration) error {
	if name == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidTimer)
	}
	if interval <= 0 {
		return fmt.Errorf("%w: interval must be positive", ErrInvalidTimer)
	}
	return nil
}
