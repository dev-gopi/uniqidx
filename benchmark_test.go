package uniqidx

import (
	"context"
	"testing"
	"time"
)

func BenchmarkNext(b *testing.B) {
	benchmarkNext(b, ModeOffline, VariantStandard, nil)
}

func BenchmarkNextCoordinated(b *testing.B) {
	benchmarkNext(b, ModeCoordinated, VariantStandard, benchmarkLeaseProvider())
}

func BenchmarkNextVariants(b *testing.B) {
	for variant := VariantStandard; variant <= VariantExtendedRandom; variant++ {
		b.Run(variant.String(), func(b *testing.B) {
			benchmarkNext(b, ModeOffline, variant, nil)
		})
	}
}

func BenchmarkNextParallelOffline(b *testing.B) {
	benchmarkNextParallel(b, ModeOffline, VariantStandard, nil)
}

func BenchmarkNextParallelCoordinated(b *testing.B) {
	benchmarkNextParallel(b, ModeCoordinated, VariantStandard, benchmarkLeaseProvider())
}

func benchmarkLeaseProvider() LeaseProvider {
	return staticLease{l: Lease{Token: Token69{Lo: 1}, ExpiresAt: time.Now().Add(time.Hour)}}
}

func benchmarkNext(b *testing.B, mode Mode, variant Variant, provider LeaseProvider) {
	var leaseProvider LeaseProvider
	if mode == ModeCoordinated {
		leaseProvider = provider
	}
	g, err := NewGenerator(context.Background(), Config{
		Mode:            mode,
		Variant:         variant,
		LeaseProvider:   leaseProvider,
		MaxLogicalDrift: time.Hour,
	})
	if err != nil {
		b.Fatal(err)
	}
	defer g.Close(context.Background())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := g.Next(context.Background()); err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkNextParallel(b *testing.B, mode Mode, variant Variant, provider LeaseProvider) {
	var leaseProvider LeaseProvider
	if mode == ModeCoordinated {
		leaseProvider = provider
	}
	g, err := NewGenerator(context.Background(), Config{
		Mode:            mode,
		Variant:         variant,
		LeaseProvider:   leaseProvider,
		MaxLogicalDrift: time.Hour,
	})
	if err != nil {
		b.Fatal(err)
	}
	defer g.Close(context.Background())
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := g.Next(context.Background()); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkParseText(b *testing.B) {
	id, _ := PackV1(1234567890, ModeOffline, Token69{Lo: 1}, 0)
	s := id.Text()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := ParseText(s); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkText(b *testing.B) {
	id, _ := PackV1(1234567890, ModeOffline, Token69{Lo: 1}, 0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = id.Text()
	}
}

func BenchmarkNumeric(b *testing.B) {
	id, _ := PackV1(1234567890, ModeOffline, Token69{Lo: 1}, 0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = id.Numeric()
	}
}
