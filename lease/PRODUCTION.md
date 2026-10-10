# Production lease-provider contract

`lease.Memory` is deliberately process-local. A production `LeaseProvider` must use durable, strongly consistent storage and satisfy these invariants:

1. Allocate a fresh 69-bit token atomically.
2. Persist permanent allocation history; never reuse a token, even after expiry or release.
3. Track active lease ownership and expiry separately from allocation history.
4. Renew only the same token/lease ownership record after coordinator acknowledgement.
5. Preserve or advance allocation history after backup restore. If that cannot be proven, use a fresh allocation domain.
6. Fail coordinated issuance after local lease expiry; do not silently downgrade to offline mode.

Every acquired lease includes an unguessable 16-byte ownership ID. Providers must require that ID as well as the token for renewal and release; possession of an ID's visible generator token must not be enough to extend or cancel its lease. `lease.Memory` enforces this rule for tests and examples, but its allocation history is process-local and therefore it is not a production coordinator.

A database-backed design commonly uses a permanent token registry protected by a unique constraint plus a transactionally updated active-lease table. etcd/Consul/SQL/cloud coordinators can all work when these invariants are enforced.

`lease/postgres` implements this pattern for PostgreSQL using a permanent `uniqidx_token_registry` and an active `uniqidx_active_leases` table. Run `Migrate` through your normal database migration process and use the provider only with a highly available PostgreSQL deployment whose recovery process preserves the registry.
