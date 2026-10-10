// Command example demonstrates all supported generation and verification paths.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/dev-gopi/uniqidx"
	"github.com/dev-gopi/uniqidx/lease"
)

type demoSigningKeys struct{ key []byte }

func (k demoSigningKeys) SigningKey(context.Context) (string, []byte, error) {
	return "demo-2026", k.key, nil
}
func (k demoSigningKeys) VerificationKey(context.Context, string) ([]byte, error) { return k.key, nil }
func must[T any](value T, err error) T {
	if err != nil {
		log.Fatal(err)
	}
	return value
}

func inspect(label string, id uniqidx.ID) {
	f := must(id.Fields())
	fmt.Printf("\n%s\n", label)
	fmt.Println("  raw hex:        ", id.Hex())
	fmt.Println("  raw base32hex:  ", id.Base32())
	fmt.Println("  raw base64url:  ", id.Base64URL())
	fmt.Println("  text / Verhoeff:", id.Text())
	fmt.Println("  text / Luhn:    ", id.TextWith(uniqidx.ChecksumLuhn))
	fmt.Println("  text / both:    ", id.TextWith(uniqidx.ChecksumBoth))
	fmt.Println("  numeric / V:    ", id.Numeric())
	fmt.Println("  numeric / both: ", id.NumericWith(uniqidx.ChecksumBoth))
	fmt.Printf("  fields: version=%d mode=%d time=%s sequence=%d token=%02x%016x\n", f.Version, f.Mode, must(id.V1Time()).Format(time.RFC3339Nano), f.Sequence, f.Token.Hi, f.Token.Lo)
}

func main() {
	ctx := context.Background()
	secret := []byte("0123456789abcdef0123456789abcdef") // Demo only: use a KMS/Vault key in production.

	offline := must(uniqidx.NewGenerator(ctx, uniqidx.Config{Mode: uniqidx.ModeOffline}))
	defer offline.Close(ctx)
	offlineID := must(offline.Next(ctx))
	inspect("OFFLINE ID", offlineID)
	fmt.Println("default ID verification:", uniqidx.ValidText(offlineID.Text()))

	// Memory is process-local: use a durable, strongly consistent provider in production.
	provider := lease.NewMemory(30*time.Second, nil)
	coordinated := must(uniqidx.NewGenerator(ctx, uniqidx.Config{Mode: uniqidx.ModeCoordinated, LeaseProvider: provider}))
	defer coordinated.Close(ctx)
	coordinatedID := must(coordinated.Next(ctx))
	inspect("COORDINATED ID", coordinatedID)
	if err := coordinated.Renew(ctx); err != nil {
		log.Fatal(err)
	}

	for _, text := range []string{coordinatedID.Text(), coordinatedID.TextWith(uniqidx.ChecksumLuhn), coordinatedID.TextWith(uniqidx.ChecksumBoth)} {
		parsed := must(uniqidx.ParseText(text))
		fmt.Println("parsed text equals original:", parsed == coordinatedID)
	}
	parsedNumeric := must(uniqidx.ParseNumeric(coordinatedID.Numeric()))
	parsedBinary := must(uniqidx.ParseBase64URL(coordinatedID.Base64URL()))
	fmt.Println("numeric/base64 round trips:   ", parsedNumeric == coordinatedID, parsedBinary == coordinatedID)

	hmacToken := must(uniqidx.SignHMAC(coordinatedID, "orders/public-id", uniqidx.HMACKey{ID: 1, Secret: secret}))
	verifiedHMAC := must(uniqidx.VerifyHMAC(hmacToken, "orders/public-id", uniqidx.Keyring{HMAC: map[uint32][]byte{1: secret}}))
	fmt.Println("HMAC token:                  ", hmacToken)
	fmt.Println("HMAC verified:               ", verifiedHMAC == coordinatedID)

	signed := must(uniqidx.Sign(ctx, coordinatedID, "orders/public-id", demoSigningKeys{secret}))
	verifiedSigned := must(uniqidx.Verify(ctx, signed, "orders/public-id", demoSigningKeys{secret}))
	fmt.Println("signed token:                ", signed)
	fmt.Println("signature verified:          ", verifiedSigned == coordinatedID)

	aead := must(uniqidx.NewAEADSigner(uniqidx.AESKey{ID: 1, Secret: secret}, nil))
	privateToken := must(aead.Seal(coordinatedID, "orders/private-id"))
	opened := must(uniqidx.OpenAEAD(privateToken, "orders/private-id", uniqidx.Keyring{AES: map[uint32][]byte{1: secret}}))
	fmt.Println("encrypted token:             ", privateToken)
	fmt.Println("encryption verified:         ", opened == coordinatedID)

	dbValue := must(coordinatedID.Value())
	var scanned uniqidx.ID
	if err := scanned.Scan(dbValue); err != nil {
		log.Fatal(err)
	}
	fmt.Println("database raw-byte round trip: ", scanned == coordinatedID)
}
