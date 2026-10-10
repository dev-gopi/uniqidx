# Security notes

Coordinated uniqueness depends on a lease provider that never reissues a token in its coordination domain. Offline IDs are probabilistic because distinct sessions can randomly choose the same 69-bit token. Do not reuse a coordinated token after restart.

Checksums are not authentication. `Sign` uses HMAC-SHA-256 truncated to 128 bits and binds a required application context. Keep signing keys outside source control, rotate keys, and use constant-time verification. IDs reveal approximate generation time and are not authorization credentials.
