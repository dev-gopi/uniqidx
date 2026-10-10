package lease

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"
)

func TestMemoryRequiresLeaseOwnershipID(t *testing.T) {
	// One token followed by a distinct 16-byte ownership handle.
	random := bytes.NewReader(append([]byte{0, 0, 0, 0, 0, 0, 0, 0, 1}, bytes.Repeat([]byte{2}, 16)...))
	m := NewMemory(time.Minute, random)
	l, err := m.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	forged := l
	forged.ID[0] ^= 1
	if _, err := m.RenewLease(context.Background(), forged); !errors.Is(err, ErrUnknownLease) {
		t.Fatalf("forged renewal error = %v, want %v", err, ErrUnknownLease)
	}
	if err := m.Release(context.Background(), forged); !errors.Is(err, ErrUnknownLease) {
		t.Fatalf("forged release error = %v, want %v", err, ErrUnknownLease)
	}
	if _, err := m.RenewLease(context.Background(), l); err != nil {
		t.Fatalf("owner renewal failed: %v", err)
	}
	if err := m.Release(context.Background(), l); err != nil {
		t.Fatalf("owner release failed: %v", err)
	}
}
