package uniqidx

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestPackBoundariesAndFields(t *testing.T) {
	token := Token69{Hi: 0x1f, Lo: ^uint64(0)}
	id, err := PackV1(maxTime44, ModeOffline, token, 1023)
	if err != nil {
		t.Fatal(err)
	}
	f, err := id.Fields()
	if err != nil || f.TimeMS != maxTime44 || f.Mode != ModeOffline || f.Token != token || f.Sequence != 1023 {
		t.Fatalf("unexpected fields: %#v, %v", f, err)
	}
	for _, tc := range []struct {
		name  string
		ms    uint64
		mode  Mode
		token Token69
		seq   uint16
		want  error
	}{
		{"time overflow", maxTime44 + 1, ModeOffline, Token69{}, 0, ErrTimestampRange},
		{"mode", 0, Mode(2), Token69{}, 0, ErrMode},
		{"token", 0, ModeOffline, Token69{Hi: 32}, 0, ErrTokenRange},
		{"sequence", 0, ModeOffline, Token69{}, 1024, ErrSequenceRange},
	} {
		if _, err := PackV1(tc.ms, tc.mode, tc.token, tc.seq); !errors.Is(err, tc.want) {
			t.Errorf("%s: got %v", tc.name, err)
		}
	}
	var invalid ID
	if _, err := invalid.Fields(); !errors.Is(err, ErrVersion) {
		t.Fatalf("invalid version: %v", err)
	}
}

func TestSequenceOverflowAndDrift(t *testing.T) {
	clock := fixedClock{epochV1.Add(10 * time.Second)}
	g, err := NewGenerator(context.Background(), Config{Mode: ModeOffline, Clock: clock, Random: zeroReader{}})
	if err != nil {
		t.Fatal(err)
	}
	var last ID
	for i := 0; i < 1025; i++ {
		last, err = g.Next(context.Background())
		if err != nil {
			t.Fatal(err)
		}
	}
	f, _ := last.Fields()
	if f.TimeMS != 10001 || f.Sequence != 0 {
		t.Fatalf("overflow fields: %#v", f)
	}
}

func TestFailedDriftDoesNotAdvanceState(t *testing.T) {
	clock := fixedClock{epochV1.Add(time.Second)}
	g, err := NewGenerator(context.Background(), Config{Mode: ModeOffline, Clock: clock, Random: zeroReader{}, MaxLogicalDrift: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2048; i++ {
		if _, err := g.Next(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := g.Next(context.Background()); !errors.Is(err, ErrClockDrift) {
		t.Fatalf("want drift error, got %v", err)
	}
	if _, err := g.Next(context.Background()); !errors.Is(err, ErrClockDrift) {
		t.Fatalf("failed call mutated state: %v", err)
	}
}

func TestStrictParsersRejectMutations(t *testing.T) {
	id, _ := PackV1(1, ModeOffline, Token69{Lo: 1}, 0)
	badCheck := "0"
	if id.Text()[len(id.Text())-1] == '0' {
		badCheck = "1"
	}
	for _, s := range []string{
		"ux1v-" + id.Base32() + "-" + badCheck, id.Text() + " ", "UX1V-" + id.Base32() + "-" + badCheck, "UX1V-" + id.Base32() + "-99", "UX1V-" + id.Base32() + "-X",
	} {
		if _, err := ParseText(s); err == nil {
			t.Errorf("accepted invalid text %q", s)
		}
	}
	for _, s := range []string{id.Base32()[:25], "a" + id.Base32()[1:], id.Base64URL() + "=", id.Hex()[:31]} {
		if len(s) == 25 || s[0] == 'a' {
			if _, err := ParseBase32(s); err == nil {
				t.Errorf("accepted invalid base32 %q", s)
			}
		}
	}
	numeric := id.NumericWith(ChecksumBoth)
	replacement := "0"
	if numeric[40] == '0' {
		replacement = "1"
	}
	badNumeric := numeric[:40] + replacement
	if _, err := ParseNumericWith(badNumeric, ChecksumBoth); err == nil {
		t.Fatal("accepted bad checksum")
	}
	if _, err := ParseNumericWith(numeric, Checksum(99)); err == nil {
		t.Fatal("accepted unknown checksum mode")
	}
	if !ValidText(id.Text()) || !ValidText(id.TextWith(ChecksumLuhn)) || !ValidText(id.TextWith(ChecksumBoth)) {
		t.Fatal("rejected valid canonical text")
	}
	if ValidText(id.Text()+"x") || ValidNumeric(badNumeric, ChecksumBoth) {
		t.Fatal("accepted invalid convenience input")
	}
}

func TestHMACAndAEADRejectWrongContextAndTampering(t *testing.T) {
	id, _ := PackV1(1, ModeOffline, Token69{Lo: 2}, 0)
	key := bytes.Repeat([]byte{7}, 32)
	h, err := SignHMAC(id, "orders", HMACKey{ID: 3, Secret: key})
	if err != nil {
		t.Fatal(err)
	}
	keys := Keyring{HMAC: map[uint32][]byte{3: key}, AES: map[uint32][]byte{3: key}}
	if _, err := VerifyHMAC(h, "users", keys); !errors.Is(err, ErrAuthentication) {
		t.Fatalf("wrong HMAC context: %v", err)
	}
	changed := "A"
	if h[len(h)-1] == 'A' {
		changed = "B"
	}
	if _, err := VerifyHMAC(h[:len(h)-1]+changed, "orders", keys); err == nil {
		t.Fatal("accepted changed HMAC")
	}
	a, err := NewAEADSigner(AESKey{ID: 3, Secret: key}, bytes.NewReader(make([]byte, 12)))
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, err := a.Seal(id, "orders")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := OpenAEAD(ciphertext, "users", keys); !errors.Is(err, ErrAuthentication) {
		t.Fatalf("wrong AEAD context: %v", err)
	}
	changed = "A"
	if ciphertext[len(ciphertext)-1] == 'A' {
		changed = "B"
	}
	if _, err := OpenAEAD(ciphertext[:len(ciphertext)-1]+changed, "orders", keys); err == nil {
		t.Fatal("accepted changed ciphertext")
	}
}

func TestConcurrentGeneratorHasNoDuplicates(t *testing.T) {
	g, err := NewGenerator(context.Background(), Config{Mode: ModeOffline, Clock: fixedClock{epochV1.Add(time.Second)}, Random: zeroReader{}, MaxLogicalDrift: 20 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	const workers, perWorker = 32, 250
	ids := make(chan ID, workers*perWorker)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				id, e := g.Next(context.Background())
				if e != nil {
					t.Errorf("next: %v", e)
					return
				}
				ids <- id
			}
		}()
	}
	wg.Wait()
	close(ids)
	seen := make(map[ID]struct{}, workers*perWorker)
	for id := range ids {
		if _, ok := seen[id]; ok {
			t.Fatal("duplicate ID")
		}
		seen[id] = struct{}{}
	}
	if len(seen) != workers*perWorker {
		t.Fatalf("got %d IDs", len(seen))
	}
}

func TestLeaseExpiryAndClose(t *testing.T) {
	clock := fixedClock{epochV1.Add(time.Second)}
	p := staticLease{Lease{Token: Token69{Lo: 3}, ExpiresAt: epochV1.Add(500 * time.Millisecond)}}
	g, err := NewGenerator(context.Background(), Config{Mode: ModeCoordinated, LeaseProvider: p, Clock: clock})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := g.Next(context.Background()); !errors.Is(err, ErrLeaseExpired) {
		t.Fatalf("expected expiry: %v", err)
	}
	if err := g.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := g.Next(context.Background()); !errors.Is(err, ErrClosed) {
		t.Fatalf("expected closed: %v", err)
	}
}
