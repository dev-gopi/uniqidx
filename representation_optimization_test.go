package uniqidx

import (
	"fmt"
	"math/big"
	"math/rand"
	"testing"
)

func TestDecimalConversionReference(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	for i := 0; i < 10000; i++ {
		var id ID
		r.Read(id[:])
		digits := id.decimalDigits()
		want := fmt.Sprintf("%039d", new(big.Int).SetBytes(id[:]))
		if string(digits[:]) != want {
			t.Fatalf("decimal mismatch for %x: %s != %s", id[:], digits, want)
		}
		// Set the version while retaining random timestamps, modes and tokens.
		id[5] = id[5]&0xf0 | Version1
		for _, c := range []Checksum{ChecksumVerhoeff, ChecksumLuhn, ChecksumBoth} {
			s := id.NumericWith(c)
			got, err := ParseNumericWith(s, c)
			if err != nil || got != id {
				t.Fatalf("numeric round trip: %s: %v", s, err)
			}
			got, err = ParseText(id.TextWith(c))
			if err != nil || got != id {
				t.Fatalf("text round trip: %v", err)
			}
		}
	}
}

func TestNumericOverflow(t *testing.T) {
	for _, d := range []string{
		"340282366920938463463374607431768211456", // 2^128
		"999999999999999999999999999999999999999",
	} {
		for _, c := range []Checksum{ChecksumVerhoeff, ChecksumLuhn, ChecksumBoth} {
			s := d
			if c != ChecksumLuhn {
				s += string(verhoeffDigitBytes([]byte(s)))
			}
			if c != ChecksumVerhoeff {
				s += string(luhnDigitBytes([]byte(s)))
			}
			if _, err := ParseNumericWith(s, c); err == nil {
				t.Fatalf("accepted overflow %s", s)
			}
		}
	}
}

func BenchmarkParseNumeric(b *testing.B) {
	id, _ := PackV1(1234567890, ModeOffline, Token69{Lo: 1}, 0)
	s := id.Numeric()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := ParseNumeric(s); err != nil {
			b.Fatal(err)
		}
	}
}
