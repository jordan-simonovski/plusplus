# Karma-war snark

## Goal

When people give/take karma to each other in quick succession, the bot adds a
short snarky line to its reply. Applies to individuals and to user-group awards.

## Trigger window

5 minutes. Best-effort, comedic flavor — not exactly-once.

## Patterns (one snark line max, in priority order)

1. **Reciprocal** (individuals): a prior interaction with actor/target swapped,
   within the window. Vibe by the two signs:
   - `++ / ++` → "karma cartel"
   - `-- / --` → "feud" ("settle down, you two")
   - mixed → "retaliation"
2. **Group train**: ≥2 prior actions targeting the *same group* in the window
   (fires on the 3rd award to that group, by anyone — including members awarding
   the group they belong to).
3. **Rapid repeat**: same actor → same target, same sign, ≥2 priors (3rd
   identical hit). Farming (`+`) / pile-on (`-`).
4. **Burst**: same actor hitting ≥3 distinct targets in the window ("spray").

Per-key cooldown (~60s) prevents snarking on every message of a long war.

Copy is tier-sliced by the existing 1–10 snark level (low = dry, high =
unhinged), distinct per vibe. No on/off flag; rides the snark level.

## State

One sentinel item per team in the existing karma table: PK `team_id`,
SK `#recent`. No `karma_total` attribute → invisible to the sparse leaderboard
GSI. Holds JSON: a window-trimmed list of `{actor, target, isGroup, sign,
unixAt}` plus a `cooldowns` map (`patternKey → lastFiredUnix`).

Cost: +1 read, +1 write per *individual* karma action (and once per group
award). Concurrency is last-writer-wins; no transactions.

## Components

- `internal/domain/karma_war.go`: `Interaction`, `KarmaWar` detector with
  `Observe(ctx, Interaction) (snarkLine string, err error)` and a separate
  classify step that's pure/table-testable. New `InteractionStore` interface
  (`LoadRecent`/`SaveRecent`).
- `internal/domain/snark.go`: new tiered snark pools per vibe.
- `KarmaService`: optional `*KarmaWar` (nil-safe). In `HandleAction`, after a
  successful apply and **only when `!GroupBroadcast`**, observe + append snark.
  Group fan-out does NOT record per-member interactions (avoids false burst).
- `EventsProcessor`: after the member loop in `handleSubteamKarma`, calls
  `ObserveGroupAward(...)` (small interface satisfied by `KarmaService`) and
  appends one line to the combined reply.
- `cmd/server/main.go`: build `DynamoInteractionStore` + `KarmaWar`, inject.

## Thresholds

Reciprocal fires on the 2nd (reply-back); repeat/train on the 3rd; burst at 3
distinct targets. Window 5m, cooldown 60s.

## Testing

Table-driven domain tests per pattern/vibe/cooldown with an in-memory
`InteractionStore` fake. Existing `fakeRepo` untouched (detector is separate and
nil in existing tests).
