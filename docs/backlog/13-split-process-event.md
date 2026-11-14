# 13. Split `HandleEvent`

Depends on 12. This is a refactor: behaviour must not change.

## Problem

`EventsProcessor.HandleEvent` in `internal/slack/events_processor.go` is about 130 lines (it was `ProcessEvent`; item 12 renamed it and moved signature checking into `VerifiedBody`)
and does five jobs in one body: decode the envelope, answer URL verification, route by
event type, filter out events that must not give karma, and run the typed karma loop
including replies. Each feature so far (reactions, user groups, target recording) has
added another branch. The typed karma logic can only be tested by building a signed HTTP
request, and it receives `*http.Request` where it needs a context (`handleSubteamKarma`
takes the request just to call `r.Context()`).

The reaction path already has the right shape: `HandleEvent` calls
`handleReaction(ctx, envelope) error` and maps the error to a status.

## Change

1. Extract `handleKarmaMessage(ctx context.Context, envelope EventEnvelope) error`
   containing everything from parsing segments to posting replies. It returns an error;
   `HandleEvent` maps it to a status.
2. Today different failures return different statuses. For a single-user target: 500
   when karma fails, 502 when the reply fails. 500 when `RecordKarmaTargets` fails. For a
   user-group target: 502 for every error, including a karma failure, with the error text
   as the body. Preserve each of these exactly, with a small typed error or sentinel that
   `HandleEvent` checks with `errors.Is`. Do not flatten them into one status, and do not
   "fix" the user-group case here.
3. Extract the filters into one function,
   `karmaEventIgnored(envelope EventEnvelope) bool`, covering the unsupported type,
   subtype, bot and duplicate-of-app-mention checks, and keep each existing comment next
   to its condition.
4. Change `handleSubteamKarma` to take `ctx` instead of `*http.Request`.
5. `HandleEvent` should end up as: decode, URL verification, then a `switch` on event
   type that calls one handler per case.

Move code; do not rewrite it. Keep function order in the file so the diff reads as moves.

## Tests

- Before refactoring, add a test for the one status no existing test pins: a failing
  karma service on a user-group award returns 502. Commit it first so it is seen passing
  against the old code.
- All existing tests in `events_processor_test.go` and `reactions_test.go` pass unchanged.
  If a test has to change, the refactor changed behaviour: stop and find out why.
- Add two direct tests of `handleKarmaMessage` without HTTP: one target applied and
  replied to; a failing karma service returns the error class that maps to 500.

## Out of scope

Any behaviour change, including fixing things you notice on the way. Note them in the PR
description instead.

## Done when

- `HandleEvent` is under 50 lines.
- No function in the file takes `*http.Request` except `HandleEvent`.
- No changeset.
