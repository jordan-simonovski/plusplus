# PlusPlus: guide for coding agents

PlusPlus is a Slack karma bot written in Go. It runs as one long-lived container, stores
everything in DynamoDB, and is distributed two ways: a hosted multi-workspace app on the
Slack Marketplace, and a self-hosted container image. Changes that are fine for one can
break the other, so read "Rules that must hold" before editing.

## Commands

```sh
gofmt -l .                # must print nothing
go vet ./...
golangci-lint run ./...   # default linters, v2.x
go test -race ./...       # unit tests, no network or database needed
make up                   # DynamoDB Local on :8000 plus the app on :8080
make test-integration     # persistence tests; needs DynamoDB Local running
make run                  # run the server against DynamoDB Local with dev secrets
```

Run the first four before every commit. Run `make test-integration` whenever you touch
`internal/persistence`, because the default test run does not exercise DynamoDB at all.

## Where things live

| Path | Holds | May import |
| --- | --- | --- |
| `cmd/server` | Wiring only: config, clients, handlers, HTTP server | everything |
| `cmd/migrate` | One-off Postgres to DynamoDB migration tool | config, persistence |
| `internal/config` | Environment variables and their validation | crypto |
| `internal/domain` | Karma rules, karma service, reaction tiers, karma-war detection, all reply copy | standard library only |
| `internal/parser` | Turns message text into karma segments (`<@U…> ++`) | standard library only |
| `internal/slack` | Slack HTTP processors (events, commands, interactions, OAuth), signature check, Web API client, settings services | domain, parser |
| `internal/persistence` | DynamoDB repositories and table creation | domain, config, crypto |
| `internal/http` | Mux, body size cap, health check | standard library only |
| `internal/crypto` | AES-256-GCM for bot tokens at rest | standard library only |
| `landing/` | Static marketing and legal site, generated changelog page | n/a |
| `terraform/` | Production AWS infrastructure | n/a |

`internal/domain` and `internal/parser` are pure. Keep Slack, HTTP and AWS types out of
them. Interfaces are declared by the package that consumes them (see the small interfaces
at the top of `internal/slack/events_processor.go`), and optional collaborators are
attached with `With…` methods in `cmd/server/main.go`.

## How a karma message flows

1. Slack posts to `/slack/events`. `EventsProcessor.ProcessEvent` reads the raw body and
   verifies the signature before parsing anything.
2. `parser.ParseKarmaSegments` extracts targets and symbol runs. The bot's own mention is
   stripped first.
3. `domain.KarmaService.HandleAction` applies the rules (minimum two symbols, per-action
   cap, no self-karma) and calls `KarmaRepository.ApplyDelta`.
4. The reply text comes from `internal/domain` and is posted with the workspace's own bot
   token through `TeamResolvingClient`.

Reactions take the same route through `handleReaction`, with per-reactor state in
`DynamoReactionStore` so that removing a reaction undoes exactly what adding it gave.

## Rules that must hold

Each rule protects users or operators. If a task seems to require breaking one, stop and
say so instead of working around it.

1. **Verify before parsing.** Every Slack-facing handler checks the request signature
   against the raw body before it decodes or acts on the payload. Unsigned requests get
   401 and no side effects.
2. **Workspaces are isolated.** Every stored row is keyed by `team_id`, and every read and
   write uses the team ID from the verified payload. In OAuth mode a workspace with no
   stored token fails; it never borrows the fallback token or another workspace's.
3. **Secrets stay secret.** Bot tokens are stored only as ciphertext. Never log tokens,
   the signing secret, the encryption key, OAuth codes, or message text.
4. **Karma changes go through `KarmaService.HandleAction`.** That is where the cap and the
   self-karma rules live. `ApplyDelta` is one atomic DynamoDB `ADD`; do not replace it with
   read-then-write.
5. **Bookkeeping rows carry no `karma_total`.** The karma table also holds rows whose
   `user_id` starts with `#` (`#recent`, `#rx#…`). The leaderboard index is sparse on
   `karma_total`, so a bookkeeping row with that attribute would appear on leaderboards.
6. **Reactions are symmetric.** A removed reaction takes back the tier that was stored
   when it was added, even if the emoji mapping changed in between.
7. **Slack retries must not double-apply.** Slack re-delivers an event when it gets no
   2xx within three seconds or gets a 5xx. Any new event handling has to be safe to
   receive twice.
8. **Scopes and events need the owner's approval.** The hosted app's scope set is
   reviewed by Slack. Do not add scopes or event subscriptions to
   `slack-app-manifest.yaml` on your own, and keep `slack-app-manifest.self-hosted.yaml`
   consistent with it.
9. **Slash commands always answer.** `help` and unknown input return the usage text (a
   Marketplace requirement). The legacy `/leaderboard` and `/settings` names keep working
   for self-hosters on the old manifest.
10. **Self-hosters upgrade by pulling an image.** A new required environment variable, a
    changed default, a new table attribute or a new IAM permission is a breaking change
    for them. Update `docs/self-hosting.md` and say so in the changeset.

## Conventions

- Standard library first. Do not add a dependency without asking; the module has two
  direct dependencies (AWS SDK, pgx for the migration tool) and that is deliberate.
- Wrap errors with `%w` and context (`fmt.Errorf("apply karma delta: %w", err)`). Text
  shown to Slack users comes from `InputError` or from `internal/domain`, never from a
  raw error.
- Tests live beside the code, use hand-written fakes (see
  `internal/slack/events_processor_test.go`), and stub HTTP with a `RoundTripper` (see
  `roundTripFunc` in `internal/slack/oauth_callback_test.go`). No mocking or assertion
  libraries.
- A comment starting `ponytail:` marks a deliberate simplification and names its limit.
  Leave those alone unless your task is to lift that limit.
- Comments explain why, not what. Match the density of the surrounding file.
- All user-facing bot copy lives in `internal/domain` (`messages.go`, `snark.go`) or in the
  usage text in `internal/slack/commands_processor.go`.

## Shipping a change

1. Branch from `main`. One logical change per PR.
2. If users or self-hosters would notice the change, add a changeset with `npx changeset`.
   Changeset text is a release note for users: say what changed for them, with no
   implementation or review detail. Refactors, tests, CI and docs need no changeset.
3. Run the commands above.
4. Follow `CLAUDE.md` for the review step before opening the PR.
5. Commits and PR descriptions carry no AI-attribution lines.

Leave these to the owner unless the task names them:

- `terraform/`: merging to `main` applies it to production automatically.
- `make release`, version bumps, and anything that pushes to `main`.
- `landing/privacy.html` and `landing/terms.html`: legal text.
- Slack app configuration (see rule 8).

## Picking up backlog work

Planned work is in `docs/backlog/`. Start with `docs/backlog/README.md`, which gives the
order and says which items can run in parallel. Each item is one file and one PR. Delete
the item's file and its row in the index in the PR that completes it. If an item is marked
"owner decision", implement the stated default and call the decision out at the top of the
PR description.
