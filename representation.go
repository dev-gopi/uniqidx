package uniqidx

import (
	"encoding/base32"
	"math/big"
	"strings"
)

func (id ID) Numeric() string { return id.NumericWith(ChecksumVerhoeff) }
func (id ID) NumericWith(c Checksum) string {
	d := new(big.Int).SetBytes(id[:]).Text(10)
	d = strings.Repeat("0", 39-len(d)) + d
	switch c {
	case ChecksumVerhoeff:
		return d + string(VerhoeffDigit(d))
	case ChecksumLuhn:
		return d + string(LuhnDigit(d))
	case ChecksumBoth:
		v := string(VerhoeffDigit(d))
		return d + v + string(LuhnDigit(d+v))
	default:
		panic("uniqidx: invalid checksum")
	}
}
func ParseNumeric(s string) (ID, error) { return ParseNumericWith(s, ChecksumVerhoeff) }
func ParseNumericWith(s string, c Checksum) (ID, error) {
	if c != ChecksumVerhoeff && c != ChecksumLuhn && c != ChecksumBoth {
		return ID{}, ErrInvalidID
	}
	want := 40
	if c == ChecksumBoth {
		want = 41
	}
	if len(s) != want || !allDigits(s) {
		return ID{}, ErrInvalidID
	}
	d := s[:39]
	if (c == ChecksumVerhoeff && (!VerhoeffValid(s))) || (c == ChecksumLuhn && (!LuhnValid(s))) || (c == ChecksumBoth && (!LuhnValid(s) || !VerhoeffValid(s[:40]))) {
		return ID{}, ErrInvalidID
	}
	n, ok := new(big.Int).SetString(d, 10)
	if !ok {
		return ID{}, ErrInvalidID
	}
	id, e := parseBytesV1(n.FillBytes(make([]byte, 16)))
	if e != nil || id.NumericWith(c) != s {
		return ID{}, ErrInvalidID
	}
	return id, nil
}
func (id ID) Text() string { return id.TextWith(ChecksumVerhoeff) }
func (id ID) TextWith(c Checksum) string {
	core := base32.HexEncoding.WithPadding(base32.NoPadding).EncodeToString(id[:])
	switch c {
	case ChecksumVerhoeff:
		return "UX1V-" + core + "-" + id.NumericWith(c)[39:]
	case ChecksumLuhn:
		return "UX1L-" + core + "-" + id.NumericWith(c)[39:]
	case ChecksumBoth:
		return "UX1B-" + core + "-" + id.NumericWith(c)[39:]
	default:
		panic("uniqidx: invalid checksum")
	}
}
func ParseText(s string) (ID, error) {
	// Canonical forms are 33 bytes (V/L) or 34 bytes (both). Check before
	// splitting so hostile oversized input cannot force allocations.
	if len(s) != 33 && len(s) != 34 {
		return ID{}, ErrInvalidID
	}
	p := strings.Split(s, "-")
	if len(p) != 3 || len(p[1]) != 26 {
		return ID{}, ErrInvalidID
	}
	var c Checksum
	switch p[0] {
	case "UX1V":
		c = ChecksumVerhoeff
	case "UX1L":
		c = ChecksumLuhn
	case "UX1B":
		c = ChecksumBoth
	default:
		return ID{}, ErrInvalidID
	}
	if len(p[2]) != map[Checksum]int{ChecksumVerhoeff: 1, ChecksumLuhn: 1, ChecksumBoth: 2}[c] {
		return ID{}, ErrInvalidID
	}
	b, e := base32.HexEncoding.WithPadding(base32.NoPadding).DecodeString(p[1])
	if e != nil || base32.HexEncoding.WithPadding(base32.NoPadding).EncodeToString(b) != p[1] {
		return ID{}, ErrInvalidID
	}
	id, e := parseBytesV1(b)
	if e != nil || id.TextWith(c) != s {
		return ID{}, ErrInvalidID
	}
	return id, nil
}

// ValidText reports whether s is a complete canonical UniqID-X public ID.
// It validates the UX1 prefix, Base32hex payload, version, and its declared
// Verhoeff, Luhn, or combined checksum. It does not authenticate the issuer.
func ValidText(s string) bool {
	_, err := ParseText(s)
	return err == nil
}

// ValidNumeric reports whether s is a canonical numeric ID with the selected
// checksum scheme. Use ValidText for self-describing UX1V/UX1L/UX1B values.
func ValidNumeric(s string, checksum Checksum) bool {
	_, err := ParseNumericWith(s, checksum)
	return err == nil
}
func parseBytesV1(b []byte) (ID, error) {
	var id ID
	if len(b) != 16 {
		return id, ErrInvalidID
	}
	copy(id[:], b)
	_, e := id.Fields()
	return id, e
}
