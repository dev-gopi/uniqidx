// Package lease supplies a process-local LeaseProvider for tests and examples.
// It is not a distributed coordinator and must not be used across processes.
package lease

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/dev-gopi/uniqidx"
)

var (
	ErrUnknownLease   = errors.New("uniqidx/lease: unknown lease")
	ErrTokenExhausted = errors.New("uniqidx/lease: random source repeatedly returned an allocated token")
)

type Memory struct {
	mu     sync.Mutex
	ttl    time.Duration
	random io.Reader
	active map[uniqidx.Token69]time.Time
	issued map[uniqidx.Token69]struct{}
}

func NewMemory(ttl time.Duration, random io.Reader) *Memory {
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	if random == nil {
		random = rand.Reader
	}
	return &Memory{ttl: ttl, random: random, active: make(map[uniqidx.Token69]time.Time), issued: make(map[uniqidx.Token69]struct{})}
}
func (m *Memory) Acquire(ctx context.Context) (uniqidx.Lease, error) {
	if e := ctx.Err(); e != nil {
		return uniqidx.Lease{}, e
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	// Expired active entries have no further liveness meaning. `issued` is
	// deliberately never pruned: reusing a token would violate uniqueness.
	for token, expires := range m.active {
		if !now.Before(expires) {
			delete(m.active, token)
		}
	}
	for attempts := 0; attempts < 1024; attempts++ {
		var b [9]byte
		if _, e := io.ReadFull(m.random, b[:]); e != nil {
			return uniqidx.Lease{}, e
		}
		t := uniqidx.Token69{Hi: b[0] & 31, Lo: binary.BigEndian.Uint64(b[1:])}
		if _, used := m.issued[t]; used {
			continue
		}
		expires := now.Add(m.ttl)
		m.issued[t] = struct{}{}
		m.active[t] = expires
		return uniqidx.Lease{Token: t, ExpiresAt: expires}, nil
	}
	return uniqidx.Lease{}, ErrTokenExhausted
}
func (m *Memory) Renew(ctx context.Context, l uniqidx.Lease) error {
	_, e := m.RenewLease(ctx, l)
	return e
}
func (m *Memory) RenewLease(ctx context.Context, l uniqidx.Lease) (uniqidx.Lease, error) {
	if e := ctx.Err(); e != nil {
		return uniqidx.Lease{}, e
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	expires, ok := m.active[l.Token]
	if !ok || !time.Now().Before(expires) {
		delete(m.active, l.Token)
		return uniqidx.Lease{}, ErrUnknownLease
	}
	expires = time.Now().Add(m.ttl)
	m.active[l.Token] = expires
	l.ExpiresAt = expires
	return l, nil
}
func (m *Memory) Release(ctx context.Context, l uniqidx.Lease) error {
	if e := ctx.Err(); e != nil {
		return e
	}
	m.mu.Lock()
	delete(m.active, l.Token)
	m.mu.Unlock()
	return nil
}
