// Package uniqidx implements the UniqID-X v1 128-bit identifier format.
package uniqidx

import (
	"database/sql/driver"
	"encoding/base32"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

const maxTime44 = (uint64(1) << 44) - 1

var (
	epochV1           = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	ErrInvalidID      = errors.New("uniqidx: invalid ID")
	ErrTimestampRange = errors.New("uniqidx: timestamp out of range")
)

type ID [16]byte

func (id ID) ValidateStructure() error { _, e := id.Fields(); return e }
func (id ID) TimeMS() uint64 {
	f, e := id.Fields()
	if e != nil {
		return 0
	}
	return f.TimeMS
}
func (id ID) Time() time.Time          { t, _ := id.V1Time(); return t }
func (id ID) Sequence() (uint32, bool) { f, e := id.Fields(); return uint32(f.Sequence), e == nil }
func (id ID) Hex() string              { return hex.EncodeToString(id[:]) }
func (id ID) Base32() string {
	return base32.HexEncoding.WithPadding(base32.NoPadding).EncodeToString(id[:])
}
func (id ID) Base64URL() string { return base64.RawURLEncoding.EncodeToString(id[:]) }
func (id ID) String() string    { return id.Text() }

func parseBytes(b []byte) (ID, error) { return parseBytesV1(b) }
func ParseHex(s string) (ID, error) {
	if len(s) != 32 {
		return ID{}, ErrInvalidID
	}
	b, e := hex.DecodeString(s)
	if e != nil || hex.EncodeToString(b) != s {
		return ID{}, ErrInvalidID
	}
	return parseBytesV1(b)
}
func ParseBase32(s string) (ID, error) { return ParseBase32V1(s) }
func ParseBase64URL(s string) (ID, error) {
	if len(s) != 22 {
		return ID{}, ErrInvalidID
	}
	b, e := base64.RawURLEncoding.DecodeString(s)
	if e != nil || base64.RawURLEncoding.EncodeToString(b) != s {
		return ID{}, ErrInvalidID
	}
	return parseBytesV1(b)
}

// Decimal is an alias for the canonical combined-checksum numeric form.
func (id ID) Decimal() string              { return id.NumericWith(ChecksumBoth) }
func ParseDecimal(s string) (ID, error)    { return ParseNumericWith(s, ChecksumBoth) }
func (id ID) MarshalText() ([]byte, error) { return []byte(id.Text()), nil }
func (id *ID) UnmarshalText(text []byte) error {
	v, e := ParseText(string(text))
	if e == nil {
		*id = v
	}
	return e
}
func (id ID) Value() (driver.Value, error) { return id[:], nil }
func (id *ID) Scan(src any) error {
	switch v := src.(type) {
	case []byte:
		x, e := parseBytesV1(v)
		if e == nil {
			*id = x
		}
		return e
	case string:
		x, e := ParseText(v)
		if e == nil {
			*id = x
		}
		return e
	default:
		return fmt.Errorf("uniqidx: cannot scan %T", src)
	}
}
