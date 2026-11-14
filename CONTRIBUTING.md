# Contributing to PlusPlus

Thanks for helping. Bug reports, fixes and small improvements are all welcome.

## Before you start

For anything bigger than a small fix, open an issue first so we can agree on the
approach. Changes that need a new Slack permission (scope or event) especially need
discussing: the hosted app's permissions are reviewed by Slack.

## Prerequisites

- Go, at the version in [`go.mod`](go.mod)
- Docker, for DynamoDB Local
- Node, only if your change needs a changeset (see below)
- [golangci-lint](https://golangci-lint.run/) at the version pinned in
  [`.github/workflows/go.yml`](.github/workflows/go.yml):
  `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@<version>`

## Run it locally

```sh
make up                # DynamoDB Local on :8000 and the app on :8080
make run               # or run the server directly against DynamoDB Local
make test-integration  # DynamoDB tests; needs DynamoDB Local running
```

To try it in Slack you need your own Slack app; see
[Slack app for development](README.md#slack-app-for-development) and [`docs/self-hosting.md`](docs/self-hosting.md).

## Before opening a pull request

```sh
make check   # gofmt, go vet, golangci-lint, and the unit tests with -race
```

CI runs the same checks, plus the DynamoDB integration tests and `govulncheck`. If you
touch `internal/persistence`, run `make test-integration` too.

## Changesets

If users or self-hosters would notice your change, add a changeset:

```sh
npx changeset
```

Write it for them: what changed and anything they need to do, not how it was built.
Refactors, tests, CI and docs changes don't need one. Merged changesets are collected
into a release pull request automatically.

## Where things live

[`AGENTS.md`](AGENTS.md) has the package layout, how a karma message flows through the
code, the rules every change must keep (signature verification first, workspace
isolation, no secrets in logs, and so on) and the code conventions. Read it before a
non-trivial change.

## Coding agents

Using a coding agent is fine. Point it at [`AGENTS.md`](AGENTS.md), which is written for
it, and review what it produces as you would your own code.

## Conduct and security

This project follows the [Code of Conduct](CODE_OF_CONDUCT.md). Please report security
problems privately as described in [SECURITY.md](SECURITY.md), not in a public issue.
