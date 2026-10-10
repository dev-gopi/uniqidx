# PostgreSQL benchmark findings

Source: `postgres-results-2026-10-10.csv` supplied for this review. Each scheme was tested three
times with one million initial rows and 2,000 mixed operations. These are
medians, not a promise of production performance; run the same workload on the
target PostgreSQL version, storage, concurrency, and autovacuum settings before
choosing a key format.

| Key | Bulk insert rows/s | Mixed ops/s | p50 / p95 / p99 (µs) | PK index size |
|---|---:|---:|---:|---:|
| UniqID-X (`BYTEA`) | 337,137 | 1,379 | 89 / 3,517 / 7,052 | 40.6 MB |
| UUID v4 | 243,547 | 1,286 | 97 / 3,982 / 7,130 | 39.5 MB |
| UUID v7 | 361,247 | 1,982 | 102 / 1,555 / 2,331 | 31.6 MB |
| `BIGINT` | 366,624 | 2,007 | 96 / 1,492 / 2,375 | 22.5 MB |

## Storage result

There is no demonstrated storage improvement for UniqID-X in these runs. The
following are averages across the three trials; use them for storage comparisons
rather than the per-run median in the performance table above.

| Key | Primary-key index | Table + indexes |
|---|---:|---:|
| UniqID-X (`BYTEA`) | 38.76 MiB | 133.57 MiB |
| UUID v4 | 37.75 MiB | 124.25 MiB |
| UUID v7 | 30.11 MiB | 116.77 MiB |
| `BIGINT` | 21.45 MiB | 100.73 MiB |

UUID v7's primary-key index remains about 22.3% smaller than UniqID-X's, and
`BIGINT`'s is about 44.7% smaller. UniqID-X's time-sortable layout may help
write locality, but it does not remove PostgreSQL's `BYTEA` representation and
index-tuple overhead.

## Performance result

UniqID-X's median bulk-insert rate is about 7% below UUID v7 and 38%
above UUID v4 in this sample. Its median mixed-workload throughput exceeds
UUID v4 but trails UUID v7 and `BIGINT`. These are observations from only three
runs, not demonstrated effects of library optimizations. `BIGINT` has the
smallest storage footprint.

Keep `BYTEA` because UniqID-X is not an RFC UUID; changing the column to `uuid`
solely to save space would misrepresent the value to clients and tools.

The mixed-operation and tail-latency results vary substantially between runs.
For example, UniqID-X mixed p95 ranges from 1.08 ms to 5.93 ms. Treat this as a
benchmark-environment signal (cache state, checkpoints, vacuum, connection
pooling, or concurrent I/O) before attributing it to ID generation. Do not
claim an UniqID-X mixed-workload improvement until controlled reruns reproduce
one.

## Verdict

- **UUID v7:** the better UUID-style database choice in this sample: smaller
  indexes and stronger mixed-workload results.
- **`BIGINT`:** the most database-efficient option, if its central-allocation
  and feature trade-offs are acceptable.
- **UniqID-X:** choose it for its public-ID format, offline generation, and
  coordination model—not for a currently demonstrated storage or mixed-workload
  advantage.

## Next improvements to measure

1. Run five to ten randomized/interleaved repetitions and report median plus
   p95 confidence intervals. Do not assign a fixed seed to a particular scheme:
   rotate schemes through each seed, and keep warm-up and PostgreSQL settings
   consistent.
2. Capture PostgreSQL version, settings, hardware, client concurrency, batch
   size, `EXPLAIN (ANALYZE, BUFFERS)` output, WAL bytes, and checkpoint/vacuum
   activity. These explain tail latency much better than rows/second alone.
3. Separate identifier-generation time from PostgreSQL insert time, then
   benchmark the actual application shape: prepared statements, transaction
   size, payload size, secondary indexes, and concurrent readers/writers.
4. Keep the column as `BYTEA` with `CHECK (octet_length(id) = 16)` and bind
   `uniqidx.ID` directly. Avoid converting IDs to text at the database boundary.
5. Inspect index ordering and page utilization with `pgstattuple` or equivalent.
   The present results do not establish a storage benefit from time ordering.
6. If primary-key index footprint is a hard capacity constraint, choose that
   trade-off explicitly (for example, `BIGINT` with a separate public ID),
   rather than weakening UniqID-X's format contract.

## Library microbenchmark

Measured on 2026-10-10, linux/amd64, Intel Core i7-11370H. Values below
are medians of three runs using `go test -run '^$' -bench . -benchmem -count=3`.
The baseline is the existing working tree before this optimization, including
its earlier ParseText allocation reduction. Final measurements ran without
concurrent tests or fuzzing; a contended intermediate run was discarded.

| Operation | Before ns/op | After ns/op | Time reduction | After allocations |
|---|---:|---:|---:|---:|
| Next | 66.07 | 63.91 | within run variation | 0 |
| ParseText | 400.6 | 303.7 | 24% | 0 |
| Text | 307.8 | 233.9 | 24% | 1 (48 bytes) |
| Numeric | 307.0 | 228.5 | 26% | 1 (48 bytes) |
| ParseNumeric | not measured | 268.9 | not established | 0 |

Decimal formatting now extracts nine digits per limb-division pass instead of
one. Numeric parsing uses checked 128-bit arithmetic, avoiding big.Int and
rejecting oversized payloads instead of panicking. The bit layout, epoch,
generator rules, public formats and checksum algorithms are unchanged.

Validation: full test suite, race detector, 10,000 deterministic random inputs
compared with big.Int, round trips for all checksum modes, overflow regression
tests, and a five-second numeric-parser fuzz run (748,399 executions).

These local microbenchmarks do not establish PostgreSQL throughput or storage
improvements. The supplied CSV predates these changes; PostgreSQL was not rerun.
Repeat measurements on the deployment host before making performance claims.
