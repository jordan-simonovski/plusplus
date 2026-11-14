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
     with `nil` so rows written before this change still work. When the fallback
     succeeds, re-encrypt the token with the team AAD and write it back with an
     `UpdateItem` on the ciphertext attribute only. If the write fails, log it and still
     return the token. Existing rows then convert the first time each workspace is active.
3. Mark the fallback with a comment:
   `// ponytail: rows written before AAD are upgraded on first read; drop this fallback once none are left.`
4. `cmd/migrate` copies existing ciphertext without decrypting it. Check that this is
   still true and that it compiles. Those rows are legacy rows and the fallback covers
   them.

## Tests

- `internal/crypto`: round trip with AAD; decrypting with different AAD fails; decrypting
  with `nil` AAD something encrypted with AAD fails.
- Integration: a token stored for team A round-trips; copying A's ciphertext attribute
  into team B's row makes `GetBotToken(B)` return an error; a row written with `nil` AAD
  (legacy) is still readable, and after that read the row no longer decrypts with `nil`
  AAD; a legacy row with one flipped byte returns an error.

## Out of scope

Key rotation, a batch job to re-encrypt rows, a key ID in the blob.

## Done when

- Tests above pass, including `make test-integration`.
- No changeset (no visible change; existing installs keep working).
