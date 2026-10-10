# UniqID-X v1 wire format

An ID is exactly 16 bytes, serialized most-significant byte first:

| Bits | Width | Field |
|---|---:|---|
| 127..84 | 44 | logical milliseconds since 2025-01-01T00:00:00Z |
| 83..80 | 4 | version (`1`) |
| 79 | 1 | mode (`0` coordinated, `1` offline) |
| 78..10 | 69 | generator token |
| 9..0 | 10 | sequence |

Canonical Base32hex is uppercase, unpadded, and 26 characters. `Text()` uses `UX1V`, `UX1L`, or `UX1B` for Verhoeff, Luhn, or both decimal-derived check digits. Checksums catch transcription errors only; use `Sign` and `Verify` when authenticity is required.
