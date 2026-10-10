# Key management and rotation

`Sign` / `Verify` use the `SigningKeyring` interface so a production application can resolve keys from AWS KMS, Google Cloud KMS, Azure Key Vault, HashiCorp Vault, an HSM, or an internal key service.

The included `keyring.Rotating` implementation is an in-process reference for tests and small controlled deployments. It copies key material on input/output and supports a current signing key plus retained verification keys. It is not a replacement for managed secret storage.

Recommended rotation sequence:

1. Add the new key as the current signing key.
2. Keep previous key IDs available for verification during the token-retention period.
3. Retire an old key only after all tokens signed with it have expired or become irrelevant.
4. Keep 32-byte HMAC/AES secrets in managed secret storage; do not commit keys or pass them in CLI arguments.

The AEAD message limit is a global per-key deployment concern. Rotate AES-GCM keys before the system emits `2^32` messages under a key across all processes.
