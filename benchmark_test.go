package uniqidx

import (
	"context"
	"testing"
	"time"
)

func BenchmarkNext(b *testing.B) {
	g, err := NewGenerator(context.Background(), Config{Mode: ModeOffline, MaxLogicalDrift: time.Hour})
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := g.Next(context.Background()); err != nil {
			b.Fatal(err)
		}
	}
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
