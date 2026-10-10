package uniqidx

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"strings"
)

// SigningKeyring supports key rotation without embedding secrets in IDs.
type SigningKeyring interface {
	SigningKey(context.Context) (string, []byte, error)
	VerificationKey(context.Context, string) ([]byte, error)
}

func signingMessage(kid, context string, id ID) ([]byte, error) {
	if len(kid) < 1 || len(kid) > 32 || len(context) < 1 || len(context) > 255 {
		return nil, ErrToken
	}
	for _, s := range []string{kid, context} {
		for i := range s {
			if s[i] < 0x20 || s[i] > 0x7e {
				return nil, ErrToken
			}
		}
	}
	out := make([]byte, 0, 16+2+len(kid)+2+len(context)+16)
	out = append(out, "UniqID-X/SIG/1"...)
	out = append(out, 0)
	var n [2]byte
	binary.BigEndian.PutUint16(n[:], uint16(len(kid)))
	out = append(out, n[:]...)
	out = append(out, kid...)
	binary.BigEndian.PutUint16(n[:], uint16(len(context)))
	out = append(out, n[:]...)
	out = append(out, context...)
	return append(out, id[:]...), nil
}
func Sign(ctx context.Context, id ID, context string, keys SigningKeyring) (string, error) {
	if _, e := id.Fields(); e != nil {
		return "", e
	}
	if keys == nil {
		return "", ErrToken
	}
	kid, key, e := keys.SigningKey(ctx)
	if e != nil {
		return "", e
	}
	if len(key) < 32 {
		return "", ErrKeyLength
	}
	m, e := signingMessage(kid, context, id)
	if e != nil {
		return "", e
	}
	mac := hmac.New(sha256.New, key)
	mac.Write(m)
	return "ux1s." + kid + "." + id.Base32() + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)[:16]), nil
}
func Verify(ctx context.Context, token, context string, keys SigningKeyring) (ID, error) {
	var z ID
	// ux1s + dots + max kid + 26-char core + 22-char tag.
	if len(token) < 56 || len(token) > 87 {
		return z, ErrToken
	}
	p := strings.Split(token, ".")
	if len(p) != 4 || p[0] != "ux1s" {
		return z, ErrToken
	}
	id, e := ParseBase32V1(p[2])
	if e != nil {
		return z, ErrToken
	}
	// Validate the caller-controlled key ID and context before allowing a
	// keyring/KMS lookup. This keeps malformed values out of backend lookups.
	m, e := signingMessage(p[1], context, id)
	if e != nil {
		return z, e
	}
	if keys == nil {
		return z, ErrToken
	}
	tag, e := base64.RawURLEncoding.DecodeString(p[3])
	if e != nil || len(tag) != 16 || base64.RawURLEncoding.EncodeToString(tag) != p[3] {
		return z, ErrToken
	}
	key, e := keys.VerificationKey(ctx, p[1])
	if e != nil {
		return z, e
	}
	if len(key) < 32 {
		return z, ErrKeyLength
	}
	mac := hmac.New(sha256.New, key)
	mac.Write(m)
	if !hmac.Equal(tag, mac.Sum(nil)[:16]) {
		return z, ErrAuthentication
	}
	return id, nil
}
func ParseBase32V1(s string) (ID, error) {
	if len(s) != 26 {
		return ID{}, ErrInvalidID
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'A' && c <= 'V') {
			return ID{}, ErrInvalidID
		}
	}
	b, e := base32.HexEncoding.WithPadding(base32.NoPadding).DecodeString(s)
	if e != nil || base32.HexEncoding.WithPadding(base32.NoPadding).EncodeToString(b) != s {
		return ID{}, ErrInvalidID
	}
	return parseBytesV1(b)
}
