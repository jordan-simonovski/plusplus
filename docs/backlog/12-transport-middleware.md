# 12. One signature gate for Slack endpoints

Do after 03, 04, 06 and 07, which edit the same handlers.
This is a refactor: behaviour must not change.

## Problem

Three handlers start with the same fifteen lines: reject non-POST, read the whole body,
read two headers, call `VerifyRequestSignature`, respond 401. They are
`EventsProcessor.ProcessEvent`, `CommandsProcessor.ProcessCommand` and
`InteractionsProcessor.processInteraction`, all in `internal/slack`. Rule 1 in `AGENTS.md`
(verify before parsing) holds only because each copy is correct, and a fourth endpoint
would need a fourth copy.

Around them is a layer that does nothing:

- `internal/http/events_handler.go` and `commands_handler.go` wrap a processor in a
  handler whose only method calls the processor.
- `internal/http/noop_processor.go` is not used by the server.
- `InteractionsProcessor.HandleInteraction` only calls `processInteraction`.
- Each processor stores its own copy of the signing secret.
- `ProcessEvent` writes `{"ok":true}` with the same two lines in seven places.

## Change

1. In `internal/slack`, add
   `func VerifiedBody(secret string, next func(w http.ResponseWriter, r *http.Request, body []byte)) http.HandlerFunc`.
   It does the method check, the read, the verification and the 401, then calls `next`
   with the raw body.
2. Change the three processors to expose a method with that `next` signature and remove
   the signing secret from their structs and constructors.
3. In `internal/http/server.go`, make `NewServer` take plain `http.Handler` values for the
   Slack routes. Delete `events_handler.go`, `commands_handler.go`, `noop_processor.go` and
   the three handler interfaces. Register routes with method patterns
   (`"POST /slack/events"`); the mux then answers other methods with 405 itself.
4. In `cmd/server/main.go`, wrap each processor with `VerifiedBody` once.
5. Add a small `ok(w)` helper in `events_processor.go` and use it for the repeated
   acknowledgement.
6. Tests currently call the processors with a signed `*http.Request`. Keep those tests
   exercising the signature: build the handler through `VerifiedBody` in a test helper
   rather than calling the inner method with a body.

## Tests

- One table test for `VerifiedBody`: missing headers, bad signature, stale timestamp and
  wrong method never call `next`; a valid request calls it with the exact body bytes.
- `TestHandlerCapsRequestBody` in `internal/http/server_test.go` still passes. Its
  `readAllHandler` helper implements the interfaces this item deletes, so adapt the helper
  to `http.Handler`; keep both assertions.
- One test builds the real route table the way `cmd/server/main.go` does and asserts that
  an unsigned POST to each of `/slack/events`, `/slack/commands` and `/slack/interactions`
  returns 401. This is what now guards rule 1, since `NewServer` accepts any handler.
- All existing processor tests pass without loosening any assertion.

## Out of scope

Changing response bodies or status codes, apart from the mux now producing the 405.
Splitting `ProcessEvent` (item 13).

## Done when

- `grep -rn "VerifyRequestSignature(" internal --include='*.go' | grep -v _test` shows two
  lines: the definition in `signature.go` and one call, inside `VerifiedBody`.
- The three deleted files are gone and the build passes.
- No changeset.
