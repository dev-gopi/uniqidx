package uniqidx

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"io"
	"sync"
	"time"
)

const Version1 uint8 = 1

type Mode uint8

const (
	ModeCoordinated Mode = iota
	ModeOffline
)

type Checksum uint8

const (
	ChecksumVerhoeff Checksum = iota
	ChecksumLuhn
	ChecksumBoth
)

type Token69 struct {
	Hi uint8
	Lo uint64
}

func (t Token69) Valid() bool { return t.Hi <= 0x1f }

type Fields struct {
	TimeMS   uint64
	Version  uint8
	Mode     Mode
	Token    Token69
	Sequence uint16
}

var (
	ErrMode          = errors.New("uniqidx: invalid mode")
	ErrTokenRange    = errors.New("uniqidx: token outside 69-bit range")
	ErrSequenceRange = errors.New("uniqidx: sequence outside 10-bit range")
	ErrVersion       = errors.New("uniqidx: unsupported version")
	ErrClosed        = errors.New("uniqidx: generator is closed")
)

func PackV1(ms uint64, mode Mode, token Token69, seq uint16) (ID, error) {
	var id ID
	if ms >= 1<<44 {
		return id, ErrTimestampRange
	}
	if mode > ModeOffline {
		return id, ErrMode
	}
	if !token.Valid() {
		return id, ErrTokenRange
	}
	if seq >= 1<<10 {
		return id, ErrSequenceRange
	}
	top15 := uint64(token.Hi)<<10 | token.Lo>>54
	hi := ms<<20 | uint64(Version1)<<16 | uint64(mode)<<15 | top15
	lo := (token.Lo&((uint64(1)<<54)-1))<<10 | uint64(seq)
	binary.BigEndian.PutUint64(id[:8], hi)
	binary.BigEndian.PutUint64(id[8:], lo)
	return id, nil
}
func (id ID) Fields() (Fields, error) {
	hi := binary.BigEndian.Uint64(id[:8])
	lo := binary.BigEndian.Uint64(id[8:])
	v := uint8((hi >> 16) & 15)
	if v != Version1 {
		return Fields{}, ErrVersion
	}
	mode := Mode((hi >> 15) & 1)
	token := Token69{Hi: uint8((hi & 0x7fff) >> 10), Lo: (hi&0x3ff)<<54 | lo>>10}
	return Fields{TimeMS: hi >> 20, Version: v, Mode: mode, Token: token, Sequence: uint16(lo & 0x3ff)}, nil
}
func (id ID) V1Time() (time.Time, error) {
	f, e := id.Fields()
	if e != nil {
		return time.Time{}, e
	}
	return time.UnixMilli(epochV1.UnixMilli() + int64(f.TimeMS)).UTC(), nil
}

type Lease struct {
	Token Token69
	// ID is an unguessable lease ownership handle. Coordinators must require it
	// for renew/release so observing an ID token is not enough to extend a lease.
	ID        [16]byte
	ExpiresAt time.Time
}
type LeaseProvider interface {
	Acquire(context.Context) (Lease, error)
	Renew(context.Context, Lease) error
	Release(context.Context, Lease) error
}

// RenewableLeaseProvider can return a refreshed expiry after an acknowledged
// coordinator renewal. Implement it when leases carry local expiry deadlines.
type RenewableLeaseProvider interface {
	LeaseProvider
	RenewLease(context.Context, Lease) (Lease, error)
}
type Config struct {
	Mode            Mode
	LeaseProvider   LeaseProvider
	Clock           Clock
	Random          io.Reader
	MaxLogicalDrift time.Duration
}
type Generator struct {
	mu                  sync.Mutex
	mode                Mode
	token               Token69
	lease               Lease
	provider            LeaseProvider
	clock               Clock
	drift               time.Duration
	initialized, closed bool
	logical             uint64
	sequence            uint16
}

func NewGenerator(ctx context.Context, c Config) (*Generator, error) {
	if c.Clock == nil {
		c.Clock = systemClock{}
	}
	if c.MaxLogicalDrift <= 0 {
		c.MaxLogicalDrift = 5 * time.Second
	}
	g := &Generator{mode: c.Mode, provider: c.LeaseProvider, clock: c.Clock, drift: c.MaxLogicalDrift}
	if c.Mode == ModeCoordinated {
		if c.LeaseProvider == nil {
			return nil, errors.New("uniqidx: coordinated mode requires a lease provider")
		}
		l, e := c.LeaseProvider.Acquire(ctx)
		if e != nil {
			return nil, e
		}
		if !l.Token.Valid() {
			return nil, ErrTokenRange
		}
		g.lease, g.token = l, l.Token
	} else if c.Mode == ModeOffline {
		if c.Random == nil {
			c.Random = rand.Reader
		}
		var b [9]byte
		if _, e := io.ReadFull(c.Random, b[:]); e != nil {
			return nil, e
		}
		g.token = Token69{Hi: b[0] & 31, Lo: binary.BigEndian.Uint64(b[1:])}
	} else {
		return nil, ErrMode
	}
	return g, nil
}
func (g *Generator) Next(ctx context.Context) (ID, error) {
	if e := ctx.Err(); e != nil {
		return ID{}, e
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if e := ctx.Err(); e != nil {
		return ID{}, e
	}
	if g.closed {
		return ID{}, ErrClosed
	}
	now := g.clock.Now()
	if g.mode == ModeCoordinated && !g.lease.ExpiresAt.IsZero() && !now.Before(g.lease.ExpiresAt) {
		return ID{}, ErrLeaseExpired
	}
	wall, e := epochMillis(now)
	if e != nil {
		return ID{}, e
	}
	logical, sequence, initialized := g.logical, g.sequence, g.initialized
	switch {
	case !initialized:
		logical, sequence, initialized = wall, 0, true
	case wall > logical:
		logical, sequence = wall, 0
	case sequence < 1023:
		sequence++
	default:
		if logical == maxTime44 {
			return ID{}, ErrTimestampRange
		}
		logical, sequence = logical+1, 0
	}
	if logical > wall && logical-wall > uint64(g.drift/time.Millisecond) {
		return ID{}, ErrClockDrift
	}
	id, e := PackV1(logical, g.mode, g.token, sequence)
	if e != nil {
		return ID{}, e
	}
	g.logical, g.sequence, g.initialized = logical, sequence, initialized
	return id, nil
}

// Renew updates the local lease deadline only after the provider acknowledges
// a renewal. Providers that do not implement RenewableLeaseProvider retain the
// compatibility Renew method but cannot update an embedded expiry.
func (g *Generator) Renew(ctx context.Context) error {
	g.mu.Lock()
	if g.closed {
		g.mu.Unlock()
		return ErrClosed
	}
	if g.mode != ModeCoordinated {
		g.mu.Unlock()
		return ErrMode
	}
	l, p := g.lease, g.provider
	g.mu.Unlock()
	if rp, ok := p.(RenewableLeaseProvider); ok {
		fresh, err := rp.RenewLease(ctx, l)
		if err != nil {
			return err
		}
		if !fresh.Token.Valid() || fresh.Token != l.Token {
			return ErrTokenRange
		}
		g.mu.Lock()
		if !g.closed {
			g.lease = fresh
		}
		g.mu.Unlock()
		return nil
	}
	return p.Renew(ctx, l)
}
func (g *Generator) Close(ctx context.Context) error {
	g.mu.Lock()
	if g.closed {
		g.mu.Unlock()
		return nil
	}
	g.closed = true
	l, p := g.lease, g.provider
	g.mu.Unlock()
	if p != nil {
		return p.Release(ctx, l)
	}
	return nil
}
