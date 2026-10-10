// Package keyring provides an in-process, rotation-capable SigningKeyring.
// Production deployments should implement the same interface with KMS, Vault,
// HSM, or a secret-management service.
package keyring

import (
	"context"
	"errors"
	"sync"
)

var ErrUnknownKey = errors.New("uniqidx/keyring: unknown key")

type Rotating struct {
	mu      sync.RWMutex
	current string
	keys    map[string][]byte
}

func New(current string, keys map[string][]byte) (*Rotating, error) {
	r := &Rotating{keys: make(map[string][]byte)}
	for id, key := range keys {
		if len(key) < 32 {
			return nil, errors.New("uniqidx/keyring: keys must contain at least 32 bytes")
		}
		r.keys[id] = append([]byte(nil), key...)
	}
	if _, ok := r.keys[current]; !ok {
		return nil, ErrUnknownKey
	}
	r.current = current
	return r, nil
}
func (r *Rotating) SigningKey(context.Context) (string, []byte, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	key, ok := r.keys[r.current]
	if !ok {
		return "", nil, ErrUnknownKey
	}
	return r.current, append([]byte(nil), key...), nil
}
func (r *Rotating) VerificationKey(_ context.Context, id string) ([]byte, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	key, ok := r.keys[id]
	if !ok {
		return nil, ErrUnknownKey
	}
	return append([]byte(nil), key...), nil
}
func (r *Rotating) Rotate(current string, key []byte) error {
	if len(key) < 32 {
		return errors.New("uniqidx/keyring: keys must contain at least 32 bytes")
	}
	r.mu.Lock()
	r.keys[current] = append([]byte(nil), key...)
	r.current = current
	r.mu.Unlock()
	return nil
}
func (r *Rotating) Retire(id string) {
	r.mu.Lock()
	if id != r.current {
		delete(r.keys, id)
	}
	r.mu.Unlock()
}
