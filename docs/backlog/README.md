# Backlog

Work identified in a codebase audit on 2026-11-14 against `main` at `3d7936e`. Read
`AGENTS.md` first. Each file here is a self-contained brief for one PR.

Code references name functions and files, not line numbers, because the code will move.
If a brief no longer matches the code, trust the code, and say what differed in the PR.

## Order

Do them one at a time, top to bottom, merging each before starting the next. That is
always safe. An item has landed when its file no longer exists in `docs/backlog/` on
`main`; where a brief says "if item N has landed", check for the file.

| # | Item | Why it matters | Size |
| --- | --- | --- | --- |
| 11 | [Dependabot and pinned actions/images](11-supply-chain-pins.md) | Mutable tags in CI and the Dockerfile | S |
| 12 | [One signature gate for Slack endpoints](12-transport-middleware.md) | Verification is copy-pasted three times | M |
| 13 | [Split `ProcessEvent`](13-split-process-event.md) | One 130-line function mixes transport and karma logic | M |
| 15 | [Dead code and duplication sweep](15-dead-code-sweep.md) | Unused exports, duplicated constants | S |
| 16 | [Fill test gaps](16-test-gaps.md) | Interactions handler has no tests; parser has no fuzzing | M |
| 18 | [Retire `cmd/migrate`](18-retire-migrate.md) | Owner confirmation required. Drops the Postgres dependency | S |

### Running items in parallel

Only these are safe to have open at the same time as other items, once 01 and 02 have
landed:

- **11** touches only CI and image configuration.

Everything else shares `internal/slack/events_processor.go`, `cmd/server/main.go` or the
handler tests, so run those strictly in order.

Sizes: S is under about 150 changed lines, M is up to about 400.

## What every brief assumes

- The PR passes the commands in `AGENTS.md`.
- Behaviour visible to users or self-hosters gets a changeset; the brief says when.
- "Done when" lists what a reviewer will check. Put evidence for each point in the PR
  description (command output, or the name of the test that proves it).

## Looked at and left alone

These were considered and are not worth doing now. Do not pick them up without a reason
that is not listed here.

- **Renaming the Go module from `plusplus` to a full import path.** This is an application,
  not a library; nobody imports it.
- **Expiring reaction rows with a TTL.** A reaction removed after its row expired would no
  longer take its karma back. The existing `ponytail:` note in
  `internal/persistence/dynamo_reaction_repository.go` covers this.
- **Retrying Slack Web API calls on 429.** Replies are best effort and name lookups already
  fall back to a mention.
- **Making karma-war state transactional.** It is flavour text and documented as
  last-writer-wins.
- **A deeper health check that pings DynamoDB.** It would restart the container during an
  AWS blip, which helps nobody.
