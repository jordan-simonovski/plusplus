# 09. Bind token ciphertext to its workspace

Depends on 02 (needs an integration test).

## Problem

`AESEncryptor` in `internal/crypto/aesgcm.go` encrypts bot tokens with AES-256-GCM and no
additional authenticated data. The ciphertext is valid for any row. Someone who can write
to the workspaces table but does not hold the key (a leaked IAM credential, a bad
migration) can copy workspace A's ciphertext into workspace B's row. It decrypts cleanly,
and the bot then acts in A with B's events. Binding each ciphertext to its team ID turns
that into a decryption failure.

## Change

1. Change the signatures to `Encrypt(plaintext, aad []byte)` and `Decrypt(blob, aad []byte)`
   and pass `aad` through to `Seal` and `Open`.
2. In `DynamoWorkspaceRepository`:
   - `UpsertInstallation` encrypts with `aad = []byte(teamID)`.
   - `GetBotToken` decrypts with `aad = []byte(teamID)`. If that fails, it tries once more
     with `nil` so rows written before this change still work.
3. Mark the fallback with a comment:
   `// ponytail: legacy rows have no AAD and stay swappable until the workspace reinstalls; drop this fallback once all rows are rewritten.`
4. `cmd/migrate` copies existing ciphertext without decrypting it. Check that this is
   still true and that it compiles. Those rows are legacy rows and the fallback covers
   them.

## Tests

- `internal/crypto`: round trip with AAD; decrypting with different AAD fails; decrypting
  with `nil` AAD something encrypted with AAD fails.
- Integration: a token stored for team A round-trips; copying A's ciphertext attribute
  into team B's row makes `GetBotToken(B)` return an error; a row written with `nil` AAD
  (legacy) is still readable.

## Out of scope

Key rotation, re-encrypting existing rows, a key ID in the blob.

## Done when

- Tests above pass, including `make test-integration`.
- No changeset (no visible change; existing installs keep working).
