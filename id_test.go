package uniqidx

import (
	"context"
	"testing"
	"time"
)

func TestV1Vectors(t *testing.T) {
	token := Token69{Hi: 0x12, Lo: 0x3456789abcdef012}
	id, e := PackV1(1234567890, ModeCoordinated, token, 7)
	if e != nil {
		t.Fatal(e)
	}
	if id.Hex() != "000499602d2148d159e26af37bc04807" {
		t.Fatal(id.Hex())
	}
	if id.Base32() != "0029IO1D454D2MF2DBPNNG280S" {
		t.Fatal(id.Base32())
	}
	if id.NumericWith(ChecksumBoth) != "00002388001597416325170634057821298893588" {
		t.Fatal(id.NumericWith(ChecksumBoth))
	}
	if id.TextWith(ChecksumBoth) != "UX1B-0029IO1D454D2MF2DBPNNG280S-88" {
		t.Fatal(id.TextWith(ChecksumBoth))
	}
	got, e := ParseText(id.TextWith(ChecksumBoth))
	if e != nil || got != id {
		t.Fatal(e)
	}
}

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }
func TestGeneratorRollback(t *testing.T) {
	g, e := NewGenerator(context.Background(), Config{Mode: ModeOffline, Clock: fixedClock{epochV1.Add(time.Second)}, Random: zeroReader{}})
	if e != nil {
		t.Fatal(e)
	}
	a, _ := g.Next(context.Background())
	b, _ := g.Next(context.Background())
	if a.TimeMS() != b.TimeMS() {
		t.Fatal("expected logical same ms")
	}
	if s, _ := b.Sequence(); s != 1 {
		t.Fatal(s)
	}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

type staticLease struct{ l Lease }

func (s staticLease) Acquire(context.Context) (Lease, error) { return s.l, nil }
func (s staticLease) Renew(context.Context, Lease) error     { return nil }
func (s staticLease) Release(context.Context, Lease) error   { return nil }
func TestNewGenerator(t *testing.T) {
	g, e := NewGenerator(context.Background(), Config{Mode: ModeCoordinated, LeaseProvider: staticLease{Lease{Token: Token69{Lo: 5}, ExpiresAt: epochV1.Add(time.Hour)}}, Clock: fixedClock{epochV1.Add(time.Second)}})
	if e != nil {
		t.Fatal(e)
	}
	a, _ := g.Next(context.Background())
	b, _ := g.Next(context.Background())
	fa, _ := a.Fields()
	fb, _ := b.Fields()
	if fa.Sequence != 0 || fb.Sequence != 1 {
		t.Fatal(fa, fb)
	}
}

type testSigningKeys struct{ key []byte }

func (k testSigningKeys) SigningKey(context.Context) (string, []byte, error)      { return "k1", k.key, nil }
func (k testSigningKeys) VerificationKey(context.Context, string) ([]byte, error) { return k.key, nil }
func TestSigningVector(t *testing.T) {
	id, _ := PackV1(1234567890, ModeCoordinated, Token69{Hi: 0x12, Lo: 0x3456789abcdef012}, 7)
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	token, e := Sign(context.Background(), id, "payment", testSigningKeys{key})
	if e != nil {
		t.Fatal(e)
	}
	if token != "ux1s.k1.0029IO1D454D2MF2DBPNNG280S.w7qFiM2oCeYkqiVaKW-_Sg" {
		t.Fatal(token)
	}
	got, e := Verify(context.Background(), token, "payment", testSigningKeys{key})
	if e != nil || got != id {
		t.Fatal(e)
	}
}
