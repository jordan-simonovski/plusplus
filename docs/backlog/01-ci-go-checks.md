# 01. Go checks in CI

## Problem

`.github/workflows/` contains only `docker.yml` and `terraform.yml`. No workflow runs
`go test`, `go vet` or a linter on pull requests, and `docker.yml` builds and publishes
the `latest` image on every push to `main` without testing it. `make lint` runs only
`go vet`.

The tree does not currently pass the checks an agent is told to run:

- `gofmt -l .` reports `internal/domain/karma_service_test.go`.
- `golangci-lint run --max-same-issues=0 ./...` (v2.9.0, default linters) reports ten
  findings: nine unchecked `Close` calls in `cmd/migrate/dump.go`, `cmd/migrate/main.go`,
  `internal/slack/client.go` and `internal/slack/oauth_handler.go`, and an unused embedded
  field on `failingWeb` in `internal/slack/reactions_test.go`. Without the flag the linter
  hides one of the duplicates and shows nine.

`govulncheck` was not run during the audit, so its first run may report something.

## Change

1. Fix the findings above. For deferred `Close` calls write
   `defer func() { _ = x.Close() }()`. Do not add a linter config file to silence them.
2. Makefile:
   - `fmt-check`: fails when `gofmt -l .` prints anything.
   - `lint`: `go vet ./...` then `golangci-lint run ./...`.
   - `test`: add `-race`.
   - `check`: `fmt-check`, `lint`, `test`.
3. Add `.github/workflows/go.yml`, triggered by `pull_request` and by `workflow_call`
   (not by `push`: step 4 already runs it on `main`). One job: checkout,
   `actions/setup-go` with `go-version-file: go.mod`, install golangci-lint v2.9.0,
   `make check`, then `go run golang.org/x/vuln/cmd/govulncheck@latest ./...`. Give the
   workflow `permissions: contents: read`.
   To install the linter use `golangci/golangci-lint-action` at a major version that
   supports golangci-lint v2 (v7 or later; earlier majors reject v2), with
   `version: v2.9.0`. Read the action's README: if it offers an install-only mode, use it
   so that `make check` is the single place the linter runs. If it does not, let the
   action run the linter and have the workflow call `make fmt-check`, `go vet ./...` and
   `make test` instead of `make check`.
4. In `docker.yml`, add a job that calls `go.yml` (`uses: ./.github/workflows/go.yml`) and
   make the build job `needs` it, so an image is never published from a failing tree.
5. In `AGENTS.md`, replace the four separate commands under "Commands" with `make check`,
   rewrite the sentence "Run the first four before every commit" to match, and delete the
   paragraph that starts "Until backlog item 01 is done".

If `govulncheck` reports a vulnerability that a dependency bump fixes, bump it in this PR.
If it does not, leave the step in, and describe the finding in the PR.

## Out of scope

Integration tests (item 02). Pinning actions to commit SHAs (item 11).

## Done when

- `make check` exits 0 on a clean checkout.
- The `go` workflow is green on the PR.
- The docker build job lists the Go job under `needs`.
- No changeset (nothing user-visible).
