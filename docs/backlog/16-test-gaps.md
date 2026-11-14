# 16. Fill test gaps

Tests only. If a new test exposes a bug, keep the test, skip it with a one-line reason,
and report the bug in the PR description. Do not fix production code in this PR.

## Where coverage is missing

Coverage at audit time (`go test -cover ./...`): parser 90.7%, domain 73.7%, crypto 68.0%,
slack 62.4%, http 39.4%, config 0%, persistence 0% (items 05 and 02 cover the last two).

### 1. `internal/slack/interactions_processor.go` has no test file

This is the handler behind the snark dropdown in the settings panel. Add
`interactions_processor_test.go` with a fake settings service:

- An unsigned request returns 401 and does not call the service.
- A body with no `payload` field returns 400.
- A payload whose type is not `block_actions` returns 200 and does not call the service.
- `block_actions` without the `snark_level_select` action returns 200 with no call.
- A level of `0`, `11` and `abc` each return the "Invalid snark level." message.
- A valid level calls the service with the team, channel and user from the payload and
  returns the service's message.
- A service error returns 500.

### 2. `internal/slack/commands_processor.go` settings paths

Existing tests cover the leaderboard, usage and help. Add:

- `reply_mode thread` and `reply_mode channel` call the service with the right mode;
  `reply_mode sideways` returns usage.
- `snark 7` sets the level; `snark abc` returns the invalid-level message; a service
  validation error is shown to the user with status 200.
- `emoji` with no arguments describes; `emoji reset`, `emoji workspace reset` and
  `emoji 3 :fire:` reach the service with the right `workspace` flag; an `InputError` is
  shown with the example suffix; any other error shows the generic retry message and not
  the error text.
- With no reaction settings attached, `emoji` returns usage.

### 3. `internal/http/server.go`

- `/healthz` returns 200 and JSON with `status: ok`.
- With no OAuth handlers, `/slack/install` and `/slack/oauth/callback` return 503.

### 4. `internal/crypto/aesgcm.go`

- `NewAESEncryptor` and `ParseKeyBase64` reject keys of the wrong length and bad base64.
- `Decrypt` of a blob shorter than the nonce, and of a blob with one flipped bit, fails.
- Two encryptions of the same plaintext differ.

### 5. Fuzz tests

Message text comes from any Slack user, so the parser and the rules should hold their
invariants for arbitrary input. Add Go native fuzz tests with a few seed inputs each.
`go test` runs the seeds on every run; nobody needs to run the fuzzer in CI.

- `FuzzParseKarmaSegments` in `internal/parser`: never panics; every segment has exactly
  one of `UserID` or `SubteamID`, matching `^[A-Z0-9]+$`; every `SymbolRun` is non-empty
  and is either all `+` or all dash characters.
- `FuzzEvaluateKarmaAction` in `internal/domain` (fuzz the symbol run and whether actor
  equals target): never panics; an applied outcome has `1 <= |Delta| <= maxSymbols-1`;
  when actor equals target the outcome is never applied.

Run each fuzzer locally for 30 seconds (`go test -fuzz=FuzzName -fuzztime=30s ./pkg`) and
commit any failing input it finds under `testdata/fuzz/`.

## Done when

- Coverage for `internal/slack` is above 75%, `internal/http` above 80%, `internal/crypto`
  above 90%. Put the before and after numbers in the PR description.
- Both fuzzers ran for 30 seconds each; state the result.
- No changeset.
