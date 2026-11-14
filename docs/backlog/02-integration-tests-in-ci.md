# 02. Integration tests in CI

Depends on 01.

## Problem

`go test -cover ./...` reports 0.0% for `internal/persistence`. Its two tests
(`dynamo_integration_test.go`, `dynamo_reaction_integration_test.go`) are behind the
`integration` build tag and skip unless `DYNAMODB_ENDPOINT` is set. Nothing in CI runs
them, so every DynamoDB expression in the project (conditional updates, the sparse
leaderboard index, the reaction state machine) is verified only when someone remembers to
run `make test-integration` locally.

## Change

1. Add an `integration` job to `.github/workflows/go.yml` with a service container running
   `amazon/dynamodb-local` on port 8000 (pin a version tag, not `latest`, and use the same
   tag in `docker-compose.yml` so there is one version to keep current), then
   `make test-integration`.
2. Read the two existing integration tests, then add tests for whatever in this list is
   not already covered. Add only what is missing.
   - `DynamoWorkspaceRepository`: a token round-trips through `UpsertInstallation` and
     `GetBotToken`; an unknown team returns `domain.ErrNotFound`; a row encrypted with a
     different key returns an error and no token.
   - `DynamoInteractionStore`: save then load returns the same window; an unknown team
     returns an empty window; a corrupt `recent_json` value returns an empty window, not
     an error.
   - Leaderboard: after writing karma rows, a `#recent` row and a `#rx#…` row for one
     team, `GetLeaderboard` returns only the karma rows (rule 5 in `AGENTS.md`), highest
     first, and nothing from another team.
   - `ApplyDelta`: `karma_max` rises to a new peak and stays there when the total falls;
     50 concurrent `ApplyDelta(+1)` calls on one user end with a total of 50.

## Out of scope

Changing repository code. If a new test exposes a bug, leave the test in, mark it with
`t.Skip` and a one-line reason, and report the bug in the PR description.

## Done when

- The `integration` job is green on the PR.
- `make test-integration` passes locally against `make up`.
- No changeset.
