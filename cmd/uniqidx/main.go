// Command uniqidx provides local generation, validation, inspection, and
// HMAC envelope operations. It is intentionally small; production systems
// should integrate the library and use a managed key provider.
package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"os"

	"github.com/dev-gopi/uniqidx"
)

type envKeyring struct{ key []byte }

func (k envKeyring) SigningKey(context.Context) (string, []byte, error)      { return "env", k.key, nil }
func (k envKeyring) VerificationKey(context.Context, string) ([]byte, error) { return k.key, nil }

func usage() {
	fmt.Fprintln(os.Stderr, "usage: uniqidx <generate|validate|inspect|sign|verify> [id-or-token] [context]")
	fmt.Fprintln(os.Stderr, "sign/verify require a 32-byte hex UNIQIDX_HMAC_KEY environment variable.")
}
func parseID(s string) (uniqidx.ID, error) { return uniqidx.ParseText(s) }
func keyring() (envKeyring, error) {
	b, err := hex.DecodeString(os.Getenv("UNIQIDX_HMAC_KEY"))
	if err != nil || len(b) != 32 {
		return envKeyring{}, fmt.Errorf("UNIQIDX_HMAC_KEY must be exactly 64 hexadecimal characters")
	}
	return envKeyring{b}, nil
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	ctx := context.Background()
	switch os.Args[1] {
	case "generate":
		g, err := uniqidx.NewGenerator(ctx, uniqidx.Config{Mode: uniqidx.ModeOffline})
		if err != nil {
			log.Fatal(err)
		}
		defer g.Close(ctx)
		id, err := g.Next(ctx)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(id.Text())
	case "validate":
		if len(os.Args) != 3 {
			usage()
			os.Exit(2)
		}
		fmt.Println(uniqidx.ValidText(os.Args[2]))
	case "inspect":
		if len(os.Args) != 3 {
			usage()
			os.Exit(2)
		}
		id, err := parseID(os.Args[2])
		if err != nil {
			log.Fatal(err)
		}
		f, _ := id.Fields()
		t, _ := id.V1Time()
		fmt.Printf("hex=%s\nbase32=%s\nversion=%d\nmode=%d\nvariant=%d\ntime=%s\nsequence=%d\n", id.Hex(), id.Base32(), f.Version, f.Mode, f.Variant, t.Format("2006-01-02T15:04:05.000Z07:00"), f.Sequence)
	case "sign":
		if len(os.Args) != 4 {
			usage()
			os.Exit(2)
		}
		id, err := parseID(os.Args[2])
		if err != nil {
			log.Fatal(err)
		}
		keys, err := keyring()
		if err != nil {
			log.Fatal(err)
		}
		token, err := uniqidx.Sign(ctx, id, os.Args[3], keys)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(token)
	case "verify":
		if len(os.Args) != 4 {
			usage()
			os.Exit(2)
		}
		keys, err := keyring()
		if err != nil {
			log.Fatal(err)
		}
		id, err := uniqidx.Verify(ctx, os.Args[2], os.Args[3], keys)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(id.Text())
	default:
		usage()
		os.Exit(2)
	}
}
