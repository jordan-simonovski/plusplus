# 03. Make typed karma safe to retry

Depends on 02 (the new store needs an integration test).

## Problem

Slack re-delivers an event up to three times when it does not get a 2xx within three
seconds, or gets a 5xx. `EventsProcessor.ProcessEvent` in
`internal/slack/events_processor.go` does all its work before responding, and the typed
karma path (`app_mention` and `message` events) has no protection against a second
delivery. Users get karma twice in each of these cases:

- **Slow request.** A user-group award applies karma to each member in sequence (two
  DynamoDB writes and a `users.info` call per member). Past three seconds Slack retries
  while the first delivery is still running, and every member is credited again.
- **Partial failure.** A message with two targets applies the first, fails on the second
  and returns 500. Slack retries and the first target is credited again.
- **Reply failure.** Karma is applied, `chat.postMessage` fails, the handler returns 502,
  Slack retries, and the karma is applied again.

The typed path also uses `r.Context()`, so when Slack hangs up at three seconds the
in-flight work is cancelled part way through.

The reaction path does not have this problem: it skips timeout retries and its stored
per-reactor state makes a repeat delivery a no-op. Leave it as it is.

## Change

Every Events API payload carries a unique `event_id` in the outer envelope. Use it to make
typed karma at-most-once.

1. Add `EventID string` (`json:"event_id"`) to `EventEnvelope` in `internal/slack/types.go`.
2. Add a small store, declared as an interface in `internal/slack` and implemented in
   `internal/persistence` on the karma table:
   - `ClaimEvent(ctx, teamID, eventID) (claimed bool, err error)`: a `PutItem` with
     `attribute_not_exists(user_id)` on the key `team_id` / `user_id = "#evt#" + eventID`.
     A failed condition returns `false, nil`.
   - `ReleaseEvent(ctx, teamID, eventID) error`: deletes that row.
   - The row has no `karma_total` (rule 5) and an `expires_at` number attribute set to
     now plus 24 hours, in epoch seconds.
3. In the typed karma path, after parsing finds at least one segment and before the first
   `HandleAction`:
   - Claim the event. If it was already claimed, respond 200 and do nothing.
   - If claiming fails with an error, respond 500 (nothing has been applied, so a retry
     is safe).
   - If the envelope has no `event_id`, skip claiming and process as today.
4. If processing fails before any karma was persisted (`KarmaResult.ShouldPersist` was
   never true), release the claim so Slack's retry can try again. Once any karma has been
   persisted, keep the claim: losing the rest of one message is better than doubling it.
5. Run the typed path on `context.WithTimeout(context.WithoutCancel(r.Context()), 15*time.Second)`,
   the same way the reaction path does, so Slack disconnecting does not abort the work.
6. Wire the store in `cmd/server/main.go` with a `With…` method. When it is not set the
   processor behaves as it does today.
7. Expiry: enable DynamoDB TTL on `expires_at` so claim rows are removed.
   - `terraform/dynamodb.tf`: add a `ttl` block to both karma tables. Terraform is
     normally the owner's (see `AGENTS.md`); this task names it, so include it and flag it
     at the top of the PR description.
   - `EnsureTables` in `internal/persistence/dynamo.go`: after creating the karma table
     (the create path only, not when the table already exists), call `UpdateTimeToLive`.
     Log and continue if it fails.
   - `docs/self-hosting.md`: add `dynamodb:UpdateTimeToLive` to the permissions for the
     option where the app creates tables, and add the TTL setting to the option where
     tables are pre-created. State that without TTL the app still works and claim rows
     accumulate.

## Tests

Unit tests in `internal/slack` with a fake store:

- The same signed payload delivered twice applies karma once and replies once.
- A failure on the first target releases the claim; a second delivery then applies.
- A failure on the second of two targets keeps the claim; a second delivery applies
  nothing.
- A payload without `event_id` is processed.

Integration test: `ClaimEvent` returns true then false for the same ID, true again after
`ReleaseEvent`, and the row does not appear in `GetLeaderboard`.

## Out of scope

Responding to Slack before processing (acknowledge first, work in the background). That
would change failure reporting and shutdown behaviour and is not needed once retries are
harmless. Do not change the reaction path.

## Done when

- The tests above pass, and `make test-integration` passes.
- Changeset (patch): karma is no longer given twice when Slack re-sends a message.
  Mention the optional TTL setting for self-hosters.
