# UniqID-X v1 wire format

An ID is exactly 16 bytes, serialized most-significant byte first:

| Bits | Width | Field |
|---|---:|---|
| 127..84 | 44 | logical milliseconds since 2025-01-01T00:00:00Z |
| 83..80 | 4 | version/variant (`1`..`4`) |
| 79 | 1 | mode (`0` coordinated, `1` offline) |
| 78..10 | 69 | generator token |
| 9..0 | 10 | sequence |

The four-bit version/variant field encodes version 1 plus the variant:
`1` is `VariantStandard`, `2` is `VariantHighPrecision`, `3` is
`VariantHighThroughput`, and `4` is `VariantExtendedRandom`. The decoder
exposes these as separate `Version` and `Variant` fields. All 69 token bits
remain available, and IDs remain exactly 16 bytes (128 bits).

In v1, variants are generation profiles and metadata; they do not change the
wire widths. Every variant uses 44-bit millisecond time, 69-bit token, and
10-bit sequence fields. This preserves compatibility and avoids claiming extra
timestamp, sequence, or randomness capacity that is not present in 128 bits.
`Variant.Profile()` exposes these limits programmatically. High Precision
retains millisecond precision and logical ordering, High Throughput retains a
maximum of 1,024 IDs per token per logical millisecond, and Extended Random
retains the 69-bit token generated from the configured secure random source in
offline mode.

Canonical Base32hex is uppercase, unpadded, and 26 characters. `Text()` uses `UX1V`, `UX1L`, or `UX1B` for Verhoeff, Luhn, or both decimal-derived check digits. Checksums catch transcription errors only; use `Sign` and `Verify` when authenticity is required.
