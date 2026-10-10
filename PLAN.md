# UniqID-X implementation plan

## Goal

Provide 16-byte, time-sortable IDs with two explicit uniqueness models:

1. **Coordinated mode** — structural non-duplication in one allocation domain, conditional on the coordinator never reissuing a 69-bit token and each process acquiring a fresh token after restart.
2. **Offline mode** — a CSPRNG-generated 69-bit token per generator session. This is probabilistic, not an absolute guarantee.

No finite uncoordinated 128-bit format can honestly promise absolute global uniqueness. A database unique constraint is always required.

## Layers

| Need | Mechanism |
|---|---|
| Ordered 16-byte identity | big-endian 44-bit logical milliseconds |
| High rate in one process | 10-bit sequence, 1,024 IDs/logical millisecond |
| Distributed uniqueness | never-reused fenced token from a linearizable lease service |
| Offline operation | OS CSPRNG session token |
| Typo detection | Verhoeff, Luhn, or both |
| Issuer/tamper authentication | HMAC-SHA-256, truncated to 128 bits |
| Confidential public token | optional AES-256-GCM envelope |
| Final duplicate protection | database `PRIMARY KEY` / `UNIQUE` constraint |

## Delivery sequence

1. Freeze the v1 wire format in `SPEC.md` and cross-language vectors.
2. Deploy a durable, strongly consistent token allocator. Preserve its high-water history through recovery; rotate allocation domains when that history is uncertain.
3. Run generators with lease expiry and renewal. Stop coordinated issuance after expiry; explicitly choose offline fallback only where acceptable.
4. Store raw 16-byte IDs, expose `Text()` in JSON/API paths, and use signed tokens only for authenticity—not authorization.
5. Run tests, race tests, parser fuzzing, concurrency stress tests, and database integration tests before release.

## Probability claims

The only valid “nearly impossible” claim is conditional and quantified. Coordinated IDs have collision probability zero **within the model** when allocation invariants hold. Offline IDs have a 69-bit session-token birthday risk that grows with the number of independent sessions. Checksum validity does not prove issuance; a valid HMAC does prove a holder of the signing key authenticated that ID/context.
