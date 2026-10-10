package uniqidx

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync/atomic"
	"unicode/utf8"
)

const (
	hmacPrefix    = "ux1h"
	aeadPrefix    = "ux1a"
	hmacDomain    = "UniqID-X\x00v1\x00HMAC-SHA256-128\x00"
	aeadDomain    = "UniqID-X\x00v1\x00AES-256-GCM\x00"
	maxContextLen = 1024
	hmacTagBytes  = 16
	gcmNonceBytes = 12
)

var (
	ErrToken          = errors.New("uniqidx: invalid token")
	ErrKeyNotFound    = errors.New("uniqidx: key id not found")
	ErrAuthentication = errors.New("uniqidx: authentication failed")
	ErrKeyLength      = errors.New("uniqidx: invalid key length")
	ErrKeyExhausted   = errors.New("uniqidx: AEAD key message limit reached")
)

type HMACKey struct {
	ID     uint32
	Secret []byte
}
type AESKey struct {
	ID     uint32
	Secret []byte
}
type Keyring struct {
	HMAC map[uint32][]byte
	AES  map[uint32][]byte
}

func contextBytes(domain string, kid uint32, context string) ([]byte, error) {
	if len(context) == 0 || len(context) > maxContextLen || !utf8.ValidString(context) {
		return nil, ErrToken
	}
	out := make([]byte, 0, len(domain)+6+len(context))
	out = append(out, domain...)
	var n [4]byte
	binary.BigEndian.PutUint32(n[:], kid)
	out = append(out, n[:]...)
	var l [2]byte
	binary.BigEndian.PutUint16(l[:], uint16(len(context)))
	out = append(out, l[:]...)
	return append(out, context...), nil
}
func keyIDText(k uint32) string { return fmt.Sprintf("%08x", k) }
func parseKeyID(s string) (uint32, error) {
	if len(s) != 8 || strings.ToLower(s) != s {
		return 0, ErrToken
	}
	if _, e := hex.DecodeString(s); e != nil {
		return 0, ErrToken
	}
	n, e := strconv.ParseUint(s, 16, 32)
	return uint32(n), e
}

// SignHMAC creates the compact ux1h authenticated envelope.
// Deprecated: prefer Sign with a SigningKeyring and the canonical ux1s format.
func SignHMAC(id ID, context string, key HMACKey) (string, error) {
	if e := id.ValidateStructure(); e != nil {
		return "", e
	}
	if len(key.Secret) < 32 {
		return "", ErrKeyLength
	}
	pre, e := contextBytes(hmacDomain, key.ID, context)
	if e != nil {
		return "", e
	}
	m := hmac.New(sha256.New, key.Secret)
	m.Write(pre)
	m.Write(id[:])
	return hmacPrefix + "." + keyIDText(key.ID) + "." + id.Base64URL() + "." + base64.RawURLEncoding.EncodeToString(m.Sum(nil)[:16]), nil
}

// VerifyHMAC verifies a compact ux1h authenticated envelope.
// Deprecated: prefer Verify with a SigningKeyring and the canonical ux1s format.
func VerifyHMAC(token, context string, keys Keyring) (ID, error) {
	var z ID
	if len(token) != 59 {
		return z, ErrToken
	}
	p := strings.Split(token, ".")
	if len(p) != 4 || p[0] != hmacPrefix {
		return z, ErrToken
	}
	kid, e := parseKeyID(p[1])
	if e != nil {
		return z, ErrToken
	}
	key, ok := keys.HMAC[kid]
	if !ok {
		return z, ErrKeyNotFound
	}
	if len(key) < 32 {
		return z, ErrKeyLength
	}
	id, e := ParseBase64URL(p[2])
	if e != nil {
		return z, ErrToken
	}
	tag, e := base64.RawURLEncoding.DecodeString(p[3])
	if e != nil || len(tag) != 16 || base64.RawURLEncoding.EncodeToString(tag) != p[3] {
		return z, ErrToken
	}
	pre, e := contextBytes(hmacDomain, kid, context)
	if e != nil {
		return z, e
	}
	m := hmac.New(sha256.New, key)
	m.Write(pre)
	m.Write(id[:])
	if !hmac.Equal(tag, m.Sum(nil)[:16]) {
		return z, ErrAuthentication
	}
	return id, nil
}

type AEADSigner struct {
	key         AESKey
	aead        cipher.AEAD
	random      io.Reader
	used        atomic.Uint64
	maxMessages uint64
}

func NewAEADSigner(key AESKey, random io.Reader) (*AEADSigner, error) {
	if len(key.Secret) != 32 {
		return nil, ErrKeyLength
	}
	// Keep an internal copy so caller mutation cannot race with encryption or
	// silently alter the configured key after construction.
	secret := append([]byte(nil), key.Secret...)
	b, e := aes.NewCipher(secret)
	if e != nil {
		return nil, e
	}
	a, e := cipher.NewGCM(b)
	if e != nil {
		return nil, e
	}
	if random == nil {
		random = rand.Reader
	}
	key.Secret = secret
	return &AEADSigner{key: key, aead: a, random: random, maxMessages: 1 << 32}, nil
}
func (s *AEADSigner) Seal(id ID, context string) (string, error) {
	if e := id.ValidateStructure(); e != nil {
		return "", e
	}
	if s.used.Add(1) > s.maxMessages {
		return "", ErrKeyExhausted
	}
	aad, e := contextBytes(aeadDomain, s.key.ID, context)
	if e != nil {
		return "", e
	}
	nonce := make([]byte, 12)
	if _, e = io.ReadFull(s.random, nonce); e != nil {
		return "", e
	}
	body := append(nonce, s.aead.Seal(nil, nonce, id[:], aad)...)
	return aeadPrefix + "." + keyIDText(s.key.ID) + "." + base64.RawURLEncoding.EncodeToString(body), nil
}
func OpenAEAD(token, context string, keys Keyring) (ID, error) {
	var z ID
	if len(token) != 73 {
		return z, ErrToken
	}
	p := strings.Split(token, ".")
	if len(p) != 3 || p[0] != aeadPrefix {
		return z, ErrToken
	}
	kid, e := parseKeyID(p[1])
	if e != nil {
		return z, ErrToken
	}
	key, ok := keys.AES[kid]
	if !ok {
		return z, ErrKeyNotFound
	}
	if len(key) != 32 {
		return z, ErrKeyLength
	}
	body, e := base64.RawURLEncoding.DecodeString(p[2])
	if e != nil || base64.RawURLEncoding.EncodeToString(body) != p[2] || len(body) != 44 {
		return z, ErrToken
	}
	b, e := aes.NewCipher(key)
	if e != nil {
		return z, e
	}
	a, e := cipher.NewGCM(b)
	if e != nil {
		return z, e
	}
	aad, e := contextBytes(aeadDomain, kid, context)
	if e != nil {
		return z, e
	}
	pt, e := a.Open(nil, body[:12], body[12:], aad)
	if e != nil {
		return z, ErrAuthentication
	}
	return parseBytes(pt)
}
