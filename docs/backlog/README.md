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
| 04 | [Log every failure](04-log-failures.md) | Most 5xx paths return without logging the cause | M |
| 05 | [Validate config at startup](05-config-validation.md) | Bad settings start a server that silently rejects everything | S |
| 06 | [Remove the token on uninstall](06-uninstall-token-cleanup.md) | Tokens for uninstalled workspaces are kept forever | M |
| 07 | [Cap user-group karma](07-group-fanout-cap.md) | One message can trigger unbounded writes and API calls | S |
| 08 | [OAuth handler cleanup](08-oauth-cleanup.md) | Dead header-trusting code path; errors echoed to browsers | S |
| 09 | [Bind token ciphertext to its workspace](09-token-aad.md) | Stored tokens can be swapped between workspaces undetected | S |
| 10 | [Contributor and issue scaffolding](10-community-health.md) | No contributing guide, security policy or issue forms | M |
| 11 | [Dependabot and pinned actions/images](11-supply-chain-pins.md) | Mutable tags in CI and the Dockerfile | S |
| 12 | [One signature gate for Slack endpoints](12-transport-middleware.md) | Verification is copy-pasted three times | M |
| 13 | [Split `ProcessEvent`](13-split-process-event.md) | One 130-line function mixes transport and karma logic | M |
| 14 | [Collapse and test the Slack Web API client](14-slack-client.md) | Three copies of one request routine, no tests | S |
| 15 | [Dead code and duplication sweep](15-dead-code-sweep.md) | Unused exports, duplicated constants | S |
| 16 | [Fill test gaps](16-test-gaps.md) | Interactions handler has no tests; parser has no fuzzing | M |
| 18 | [Retire `cmd/migrate`](18-retire-migrate.md) | Owner confirmation required. Drops the Postgres dependency | S |
| 19 | [DynamoDB deletion protection](19-terraform-deletion-protection.md) | Owner merges this. Production tables can be deleted by a bad plan | S |

### Running items in parallel

Only these are safe to have open at the same time as other items, once 01 and 02 have
landed:

- **14** stays inside `client.go` and `team_client.go`.
- **09** touches only `internal/crypto` and the workspace repository. Not alongside 06.
- **19** touches only `terraform/dynamodb.tf`. Not alongside 03, which edits the same file.
- **11** touches only CI and image configuration.
- **10** adds new files and reorders `README.md`. Not alongside 05, 06 or 18, which also
  edit `README.md` or `docs/self-hosting.md`.

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
