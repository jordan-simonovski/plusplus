# Backlog

Work identified in a codebase audit on 2026-11-14 against `main` at `3d7936e`. Read
`AGENTS.md` first. Each file here is a self-contained brief for one PR.

Code references name functions and files, not line numbers, because the code will move.
If a brief no longer matches the code, trust the code, and say what differed in the PR.

## Order

Do them top to bottom unless the "Can run alongside" column says otherwise. Items that
share a file must not be in flight at the same time; the column lists what is safe.

| # | Item | Why it matters | Size | Can run alongside |
| --- | --- | --- | --- | --- |
| 01 | [Go checks in CI](01-ci-go-checks.md) | Nothing tests Go code on PRs; images publish untested | S | nothing (do first) |
| 02 | [Integration tests in CI](02-integration-tests-in-ci.md) | The DynamoDB layer has 0% coverage in the default run | S | 03 onward |
| 03 | [Make typed karma safe to retry](03-event-idempotency.md) | A Slack retry gives the same karma twice | M | 05, 08, 09, 10, 11 |
| 04 | [Log every failure](04-log-failures.md) | Most 5xx paths return without logging the cause | M | 10, 11 |
| 05 | [Validate config at startup](05-config-validation.md) | Bad settings start a server that silently rejects everything | S | 03, 10, 11 |
| 06 | [Delete the token on uninstall](06-uninstall-token-cleanup.md) | Tokens for uninstalled workspaces are kept forever | M | 10, 11 |
| 07 | [Cap user-group karma](07-group-fanout-cap.md) | One message can trigger unbounded writes and API calls | S | 10, 11 |
| 08 | [OAuth handler cleanup](08-oauth-cleanup.md) | Dead header-trusting code path; errors echoed to browsers | S | 03, 10, 11 |
| 09 | [Bind token ciphertext to its workspace](09-token-aad.md) | Stored tokens can be swapped between workspaces undetected | S | 03, 10, 11 |
| 10 | [Contributor and issue scaffolding](10-community-health.md) | No contributing guide, security policy or issue forms | M | any |
| 11 | [Dependabot and pinned actions/images](11-supply-chain-pins.md) | Mutable tags in CI and the Dockerfile | S | any |
| 12 | [One signature gate for Slack endpoints](12-transport-middleware.md) | Verification is copy-pasted three times | M | 10, 11 |
| 13 | [Split `ProcessEvent`](13-split-process-event.md) | One 130-line function mixes transport and karma logic | M | 10, 11 |
| 14 | [Collapse and test the Slack Web API client](14-slack-client.md) | Three copies of one request routine, no tests | S | 10, 11, 17 |
| 15 | [Dead code and duplication sweep](15-dead-code-sweep.md) | Unused exports, duplicated constants | S | 10, 11 |
| 16 | [Fill test gaps](16-test-gaps.md) | Interactions handler has no tests; parser has no fuzzing | M | 10, 11 |
| 17 | [Bound the name cache](17-name-cache-bound.md) | The cache never evicts | S | 10, 11, 14 |
| 18 | [Retire `cmd/migrate`](18-retire-migrate.md) | Owner decision. Drops the Postgres dependency | S | 10, 11 |
| 19 | [DynamoDB deletion protection](19-terraform-deletion-protection.md) | Owner runs this. Production tables can be deleted by a bad plan | S | any |

Items 03, 04, 06, 07, 12 and 13 all edit `internal/slack/events_processor.go`. Run them
one at a time, in that order.

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
