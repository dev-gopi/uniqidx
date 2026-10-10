package uniqidx

import (
	"encoding/base32"
	"encoding/binary"
	"math/bits"
)

func (id ID) Numeric() string { return id.NumericWith(ChecksumVerhoeff) }
func (id ID) NumericWith(c Checksum) string {
	d := id.decimalDigits()
	var out [41]byte
	copy(out[:39], d[:])
	switch c {
	case ChecksumVerhoeff:
		out[39] = verhoeffDigitBytes(d[:])
		return string(out[:40])
	case ChecksumLuhn:
		out[39] = luhnDigitBytes(d[:])
		return string(out[:40])
	case ChecksumBoth:
		out[39] = verhoeffDigitBytes(d[:])
		out[40] = luhnDigitBytes(out[:40])
		return string(out[:])
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
	// Accumulate the fixed-width decimal payload with explicit overflow checks.
	var hi, lo uint64
	for i := range d {
		carry, low := bits.Mul64(lo, 10)
		overflow, high := bits.Mul64(hi, 10)
		high, c1 := bits.Add64(high, carry, 0)
		low, carry = bits.Add64(low, uint64(d[i]-'0'), 0)
		high, c2 := bits.Add64(high, 0, carry)
		if overflow != 0 || c1 != 0 || c2 != 0 {
			return ID{}, ErrInvalidID
		}
		hi, lo = high, low
	}
	var id ID
	binary.BigEndian.PutUint64(id[:8], hi)
	binary.BigEndian.PutUint64(id[8:], lo)
	if _, err := id.Fields(); err != nil {
		return ID{}, ErrInvalidID
	}
	return id, nil
}
func (id ID) Text() string { return id.TextWith(ChecksumVerhoeff) }
func (id ID) TextWith(c Checksum) string {
	var out [34]byte
	copy(out[:3], "UX1")
	out[4], out[31] = '-', '-'
	base32.HexEncoding.WithPadding(base32.NoPadding).Encode(out[5:31], id[:])
	d := id.decimalDigits()
	switch c {
	case ChecksumVerhoeff:
		out[3], out[32] = 'V', verhoeffDigitBytes(d[:])
		return string(out[:33])
	case ChecksumLuhn:
		out[3], out[32] = 'L', luhnDigitBytes(d[:])
		return string(out[:33])
	case ChecksumBoth:
		out[3], out[32] = 'B', verhoeffDigitBytes(d[:])
		var checksumInput [40]byte
		copy(checksumInput[:39], d[:])
		checksumInput[39] = out[32]
		out[33] = luhnDigitBytes(checksumInput[:])
		return string(out[:])
	default:
		panic("uniqidx: invalid checksum")
	}
}

// decimalDigits returns the zero-padded decimal representation of a 128-bit
// ID. Dividing four base-2^32 limbs by 10^9 extracts nine digits per pass;
// each intermediate fits in uint64 because the remainder is below 10^9.
func (id ID) decimalDigits() [39]byte {
	words := [4]uint32{
		binary.BigEndian.Uint32(id[0:4]), binary.BigEndian.Uint32(id[4:8]),
		binary.BigEndian.Uint32(id[8:12]), binary.BigEndian.Uint32(id[12:16]),
	}
	var digits [39]byte
	pos := len(digits)
	for pos > 0 {
		var remainder uint64
		for i := range words {
			value := remainder<<32 | uint64(words[i])
			words[i] = uint32(value / 1_000_000_000)
			remainder = value % 1_000_000_000
		}
		end := pos - 9
		if end < 0 {
			end = 0
		}
		for pos > end {
			pos--
			digits[pos] = '0' + byte(remainder%10)
			remainder /= 10
		}
	}
	return digits
}
func ParseText(s string) (ID, error) {
	// Canonical forms are 33 bytes (V/L) or 34 bytes (both). Check before
	// slicing so hostile oversized input cannot force allocations.
	if len(s) != 33 && len(s) != 34 {
		return ID{}, ErrInvalidID
	}
	// The fixed layout lets us parse without strings.Split or a temporary
	// decoded byte slice. Besides being cheaper on a validation-heavy API,
	// this keeps the canonical-format checks explicit.
	if s[4] != '-' || s[31] != '-' {
		return ID{}, ErrInvalidID
	}
	var c Checksum
	switch s[:4] {
	case "UX1V":
		c = ChecksumVerhoeff
	case "UX1L":
		c = ChecksumLuhn
	case "UX1B":
		c = ChecksumBoth
	default:
		return ID{}, ErrInvalidID
	}
	checksumLen := 1
	if c == ChecksumBoth {
		checksumLen = 2
	}
	if len(s[32:]) != checksumLen {
		return ID{}, ErrInvalidID
	}
	payload := s[5:31]
	for i := range payload {
		// Decode accepts more than the published Base32hex spelling on some
		// Go versions. Enforce the canonical, uppercase alphabet ourselves.
		ch := payload[i]
		if !(ch >= '0' && ch <= '9') && !(ch >= 'A' && ch <= 'V') {
			return ID{}, ErrInvalidID
		}
	}
	// Sixteen bytes occupy 25 full Base32 characters plus three bits. The final
	// two bits must be zero; otherwise Decode would accept a non-canonical
	// spelling that re-encodes differently.
	last := payload[len(payload)-1]
	value := last - '0'
	if last >= 'A' {
		value = last - 'A' + 10
	}
	if value&3 != 0 {
		return ID{}, ErrInvalidID
	}
	var id ID
	n, e := base32.HexEncoding.WithPadding(base32.NoPadding).Decode(id[:], []byte(payload))
	if e != nil || n != len(id) {
		return ID{}, ErrInvalidID
	}
	if _, e = id.Fields(); e != nil {
		return ID{}, ErrInvalidID
	}
	// Recompute only the declared decimal checksum; the Base32 payload has
	// already been validated above.
	digits := id.decimalDigits()
	if !matchesChecksum(digits, c, s[32:]) {
		return ID{}, ErrInvalidID
	}
	return id, nil
}

func matchesChecksum(digits [39]byte, c Checksum, suffix string) bool {
	switch c {
	case ChecksumVerhoeff:
		return suffix[0] == verhoeffDigitBytes(digits[:])
	case ChecksumLuhn:
		return suffix[0] == luhnDigitBytes(digits[:])
	case ChecksumBoth:
		if suffix[0] != verhoeffDigitBytes(digits[:]) {
			return false
		}
		var checksumInput [40]byte
		copy(checksumInput[:39], digits[:])
		checksumInput[39] = suffix[0]
		return suffix[1] == luhnDigitBytes(checksumInput[:])
	default:
		return false
	}
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
