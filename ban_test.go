package exchanges

import (
	"errors"
	"testing"
	"time"
)

func TestParseAndSetBan(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		wantBan bool
	}{
		{
			name:    "binance ban message",
			err:     errors.New("Way too many requests; IP(13.113.225.45) banned until 1773103961466"),
			wantBan: true,
		},
		{
			name:    "aster ban message",
			err:     errors.New("Way too many requests; IP(1.2.3.4) banned until 1773103961466"),
			wantBan: true,
		},
		{
			name:    "normal error",
			err:     errors.New("connection timeout"),
			wantBan: false,
		},
		{
			name:    "nil error",
			err:     nil,
			wantBan: false,
		},
		{
			name:    "short number not matched",
			err:     errors.New("banned until 12345"),
			wantBan: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &BanState{}
			got := b.ParseAndSetBan(tt.err)
			if got != tt.wantBan {
				t.Errorf("ParseAndSetBan() = %v, want %v", got, tt.wantBan)
			}
		})
	}
}

func TestBanExpiry(t *testing.T) {
	b := &BanState{}

	// Set ban 200ms from now
	futureMs := time.Now().Add(200 * time.Millisecond).UnixMilli()
	b.SetBan(futureMs)

	// Should be banned now
	banned, expiry := b.IsBanned()
	if !banned {
		t.Fatal("expected to be banned")
	}
	if expiry.IsZero() {
		t.Fatal("expected non-zero expiry")
	}

	// BannedFor should be positive
	dur := b.BannedFor()
	if dur <= 0 {
		t.Fatalf("expected positive duration, got %v", dur)
	}

	// Wait for ban to expire
	time.Sleep(250 * time.Millisecond)

	// Should no longer be banned
	banned, _ = b.IsBanned()
	if banned {
		t.Fatal("expected ban to have expired")
	}

	dur = b.BannedFor()
	if dur != 0 {
		t.Fatalf("expected 0 duration after expiry, got %v", dur)
	}
}

func TestBanAlreadyExpired(t *testing.T) {
	b := &BanState{}

	// Set ban in the past
	b.SetBan(time.Now().Add(-1 * time.Second).UnixMilli())

	banned, _ := b.IsBanned()
	if banned {
		t.Fatal("past ban should not be active")
	}
}

func TestErrBanned(t *testing.T) {
	e := &ErrBanned{Until: time.Now().Add(30 * time.Second)}
	msg := e.Error()
	if msg == "" {
		t.Fatal("expected non-empty error message")
	}
	t.Log(msg) // visual check
}
