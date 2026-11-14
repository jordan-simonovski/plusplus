# 15. Dead code and duplication sweep

Do after 12 and 13 so the sweep does not collide with them. Behaviour must not change.

## Candidates

Each of these looked unused or duplicated at audit time. Confirm each with a search before
removing it, because earlier backlog items may have changed the picture. Remove a symbol
only when nothing outside its own tests refers to it. Do not lose test coverage of logic
that stays: `TestEvaluateKarmaAction` exercises the core rules, so point it at
`EvaluateKarmaActionWithLimits(input, minSymbolCount, maxSymbolCount)`, and point the
`ParseMentionAction` cases at `ParseKarmaSegments`. Delete a test only when it covers
nothing but the removed symbol.

| Where | What | Finding |
| --- | --- | --- |
| `internal/domain/karma_service.go` | `ErrUnsupportedReason` | Declared, never returned or checked |
| `internal/domain/karma_rules.go` | `EvaluateKarmaAction` | Only `EvaluateKarmaActionWithLimits` is called from non-test code |
| `internal/parser/mention_parser.go` | `ParseMentionAction`, `ParseMentionActions`, `MentionAction` | The server uses `ParseKarmaSegments`; check whether only tests use these |
| `internal/slack/settings_service.go`, `internal/persistence/dynamo_settings_repository.go` | `GetReplyMode` on both | Callers use `GetChannelSettings` |
| `internal/slack/types.go` | `SlashCommandPayload` fields `Token`, `ChannelName`, `EnterpriseID`, `EnterpriseName`; `EventEnvelope.Token`; `SlackEvent.ChannelID`; `MessageResponse.ThreadTS` | Never set or read |
| `internal/slack/types.go` | `Blocks interface{}` | Use `any` |
| `internal/persistence/dynamo_settings_repository.go` | `defaultSnarkLevel`, `minSnarkLevel`, `maxSnarkLevel` | Duplicate `domain.DefaultSnarkLevel`, `MinSnarkLevel`, `MaxSnarkLevel`; persistence already imports domain |
| `internal/domain/snark.go` | `rand.New(rand.NewSource(time.Now().UnixNano()))` on every call | Switch the file to `math/rand/v2` and call `rand.IntN` directly; it is safe for concurrent use and needs no seeding. Remove the comments that explain the per-call seeding |
| `internal/slack/reaction_settings.go` | local `abs` | Keep unless the standard library gains an integer `abs`; listed so nobody spends time on it |

If item 05 has not landed, also remove `Config.SlackAppToken` here.

## How to check

```sh
grep -rn "SymbolName" cmd internal --include='*.go'
```

Run it for each symbol and read every hit. The `unused` linter only reports unexported
symbols, so it will not find most of these.

## Tests

- `go test -race ./...` passes with no test weakened.
- `SnarkPicker` behaviour is covered by existing tests that inject a picker; confirm they
  still pass after the `math/rand/v2` change. If a test depended on a seeded sequence,
  inject the choice instead of seeding.

## Out of scope

Renaming things, moving files, changing signatures that are in use. Anything not in the
table: add it to the PR description as a suggestion instead.

## Done when

- Each row is either removed or has a one-line reason in the PR description for staying.
- `make check` (or the commands in `AGENTS.md`) passes.
- No changeset.
