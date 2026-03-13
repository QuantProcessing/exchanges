package exchanges

import (
	"fmt"
	"regexp"
	"strconv"
	"sync/atomic"
	"time"
)

// banPattern matches "banned until 1773103961466" style messages (unix ms).
var banPattern = regexp.MustCompile(`banned until (\d{13,})`)

// BanState tracks IP ban status using an atomic timestamp for lock-free access.
// Zero means not banned; non-zero is the unix-ms expiry time.
type BanState struct {
	bannedUntil atomic.Int64
}

// SetBan records a ban that expires at the given unix-ms timestamp.
func (b *BanState) SetBan(untilMs int64) {
	b.bannedUntil.Store(untilMs)
}

// IsBanned returns the current ban status and expiry time.
// Returns (false, zero) if not banned or ban has expired.
func (b *BanState) IsBanned() (bool, time.Time) {
	ms := b.bannedUntil.Load()
	if ms == 0 {
		return false, time.Time{}
	}
	expiry := time.UnixMilli(ms)
	if time.Now().After(expiry) {
		// Ban expired — clear it
		b.bannedUntil.CompareAndSwap(ms, 0)
		return false, time.Time{}
	}
	return true, expiry
}

// BannedFor returns the remaining ban duration. Returns 0 if not banned.
func (b *BanState) BannedFor() time.Duration {
	banned, expiry := b.IsBanned()
	if !banned {
		return 0
	}
	return time.Until(expiry)
}

// ParseAndSetBan checks if an error contains a "banned until" message.
// If found, records the ban and returns true.
func (b *BanState) ParseAndSetBan(err error) bool {
	if err == nil {
		return false
	}
	matches := banPattern.FindStringSubmatch(err.Error())
	if len(matches) < 2 {
		return false
	}
	ms, parseErr := strconv.ParseInt(matches[1], 10, 64)
	if parseErr != nil {
		return false
	}
	b.SetBan(ms)
	return true
}

// ErrBanned is returned when a request is blocked due to an active IP ban.
type ErrBanned struct {
	Until time.Time
}

func (e *ErrBanned) Error() string {
	return fmt.Sprintf("IP banned until %s (remaining: %s)",
		e.Until.Format("15:04:05"), time.Until(e.Until).Truncate(time.Second))
}
