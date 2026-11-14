# 04. Log every failure

Do after 03 to avoid conflicts in `events_processor.go`.

## Problem

Most paths that return a 5xx do so without logging why. An operator, hosted or
self-hosted, sees Slack retries and nothing else. Examples, all in `internal/slack`:

- `ProcessEvent`: "failed to apply karma", "failed to post message", "failed to record
  karma message", "failed to apply reaction karma", and the user-group path.
- `ProcessCommand` and its helpers: "failed to build leaderboard", "failed to load
  settings", "failed to update settings".
- `processInteraction`: "failed to update settings".
- `OAuthHandler.Callback`: a failed code exchange and "failed to save installation".

Best-effort paths also drop errors without a trace: `resolveChannelContext` ignores a
settings read failure, and `observeGroupAward` and `HandleAction` ignore karma-war store
errors.

The 11 existing `log.Printf` calls in `cmd/server` and `internal/` use free-form text, which cannot be filtered
by workspace.

## Change

1. In `cmd/server/main.go`, set the default logger to
   `slog.New(slog.NewJSONHandler(os.Stdout, nil))`.
2. Wherever a handler returns a 5xx, log once at error level before responding, with the
   cause and these attributes where known: `team` (team ID), `kind` (the event type, the
   slash command name, `interaction`, or `oauth`), and `err`.
3. Where an error is deliberately swallowed (the best-effort paths above), log it at warn
   level with the same attributes. For `HandleAction` in `internal/domain`, which must not
   import a logger-specific type, use `slog` from the standard library directly; that
   keeps the package standard-library only.
4. Convert the existing `log.Printf` calls in `cmd/server` and `internal/` to `slog` with
   attributes instead of formatted strings. Leave `log.Fatalf` at startup and everything
   in `cmd/migrate` alone.

Never log tokens, secrets, OAuth codes or message text (rule 3 in `AGENTS.md`). User and
channel IDs are fine.

## Out of scope

Request logging, metrics, tracing, log levels from configuration. Changing any response
status or body.

## Done when

- Every `http.Error` call with a 5xx status in `internal/slack` has a log call on the same
  path. List them in the PR description.
- One test in `internal/slack` swaps the default logger for one writing to a buffer,
  triggers a karma failure, and asserts the log line contains the team ID and the error
  and does not contain the message text.
- `grep -rn "log.Printf" internal cmd/server` prints nothing.
- Changeset (patch): failures are now logged as JSON lines with the workspace ID. Note for
  self-hosters that the log format changed.
