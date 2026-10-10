package uniqidx

import (
	"context"
	"testing"
)

func FuzzParseText(f *testing.F) {
	f.Add("UX1B-0029IO1D454D2MF2DBPNNG280S-88")
	f.Add("")
	f.Fuzz(func(t *testing.T, s string) {
		id, err := ParseText(s)
		if err == nil && id.TextWith(ChecksumBoth) != s && id.Text() != s && id.TextWith(ChecksumLuhn) != s {
			t.Fatalf("accepted noncanonical text %q", s)
		}
	})
}

func FuzzParseNumeric(f *testing.F) {
	f.Add("00002388001597416325170634057821298893588")
	f.Add("0")
	f.Fuzz(func(t *testing.T, s string) {
		id, err := ParseNumericWith(s, ChecksumBoth)
		if err == nil && id.NumericWith(ChecksumBoth) != s {
			t.Fatalf("accepted noncanonical number %q", s)
		}
	})
}

func FuzzSignedToken(f *testing.F) {
	f.Add("ux1s.k1.0029IO1D454D2MF2DBPNNG280S.w7qFiM2oCeYkqiVaKW-_Sg")
	f.Add("")
	key := make([]byte, 32)
	f.Fuzz(func(t *testing.T, token string) {
		_, _ = Verify(context.Background(), token, "payment", testSigningKeys{key})
	})
}
