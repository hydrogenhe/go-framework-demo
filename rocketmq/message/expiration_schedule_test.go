package message

import (
	"testing"
	"time"
)

func TestExpirationDelayLevelRoundsUp(t *testing.T) {
	cases := []struct {
		remaining time.Duration
		level     int
		invalid   bool
	}{
		{10 * time.Minute, 14, false}, {90 * time.Second, 6, false},
		{12 * time.Minute, 15, false}, {time.Minute, 5, false},
		{time.Minute + time.Nanosecond, 6, false}, {10*time.Minute - time.Millisecond, 14, false},
		{0, 1, false}, {-time.Second, 1, false}, {2 * time.Hour, 18, false},
		{2*time.Hour + time.Nanosecond, 0, true}, {7 * 24 * time.Hour, 0, true},
	}
	for _, tc := range cases {
		level, err := ExpirationDelayLevel(tc.remaining)
		if level != tc.level || (err != nil) != tc.invalid {
			t.Errorf("remaining=%v: level=%d err=%v; want level=%d invalid=%v", tc.remaining, level, err, tc.level, tc.invalid)
		}
	}
}
