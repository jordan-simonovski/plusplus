# 01. Go checks in CI

## Problem

`.github/workflows/` contains only `docker.yml` and `terraform.yml`. No workflow runs
`go test`, `go vet` or a linter on pull requests, and `docker.yml` builds and publishes
the `latest` image on every push to `main` without testing it. `make lint` runs only
`go vet`.

The tree does not currently pass the checks an agent is told to run:

- `gofmt -l .` reports `internal/domain/karma_service_test.go`.
- `golangci-lint run ./...` (v2.9.0, default linters) reports nine findings: eight
  unchecked `Close` calls in `cmd/migrate/dump.go`, `cmd/migrate/main.go` and
  `internal/slack/client.go`, and an unused embedded field on `failingWeb` in
  `internal/slack/reactions_test.go`.

`govulncheck` was not run during the audit, so its first run may report something.

## Change

1. Fix the findings above. For deferred `Close` calls write
   `defer func() { _ = x.Close() }()`. Do not add a linter config file to silence them.
2. Makefile:
   - `fmt-check`: fails when `gofmt -l .` prints anything.
   - `lint`: `go vet ./...` then `golangci-lint run ./...`.
   - `test`: add `-race`.
   - `check`: `fmt-check`, `lint`, `test`.
3. Add `.github/workflows/go.yml`, triggered by `pull_request`, by `push` to `main`, and
   by `workflow_call`. One job: checkout, `actions/setup-go` with
   `go-version-file: go.mod`, the `golangci/golangci-lint-action` pinned to v2.9.0 of the
   linter, `make check`, then `govulncheck ./...`. Give the workflow
   `permissions: contents: read`.
4. In `docker.yml`, add a job that calls `go.yml` (`uses: ./.github/workflows/go.yml`) and
   make the build job `needs` it, so an image is never published from a failing tree.
5. In `AGENTS.md`, replace the four separate commands under "Commands" with `make check`.

If `govulncheck` reports a vulnerability that a dependency bump fixes, bump it in this PR.
If it does not, leave the step in, and describe the finding in the PR.

## Out of scope

Integration tests (item 02). Pinning actions to commit SHAs (item 11).

## Done when

- `make check` exits 0 on a clean checkout.
- The `go` workflow is green on the PR.
- The docker build job lists the Go job under `needs`.
- No changeset (nothing user-visible).
