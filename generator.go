package uniqidx

import (
	"errors"
	"time"
)

var (
	ErrClockBeforeEpoch = errors.New("uniqidx: clock is before epoch")
	ErrClockDrift       = errors.New("uniqidx: logical clock exceeds maximum future drift")
	ErrLeaseExpired     = errors.New("uniqidx: worker lease is not valid")
)

// Clock makes generator time deterministic in tests.
type Clock interface{ Now() time.Time }
type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }
func epochMillis(t time.Time) (uint64, error) {
	if t.Before(epochV1) {
		return 0, ErrClockBeforeEpoch
	}
	d := t.UnixMilli() - epochV1.UnixMilli()
	if d < 0 {
		return 0, ErrClockBeforeEpoch
	}
	if uint64(d) > maxTime44 {
		return 0, ErrTimestampRange
	}
	return uint64(d), nil
}
