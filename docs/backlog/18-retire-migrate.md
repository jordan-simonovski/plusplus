# 18. Retire `cmd/migrate`

## Owner decision required before starting

`cmd/migrate` is a one-off tool that copied data from the original Postgres (Supabase)
database into DynamoDB. Do this item
only once the owner has confirmed that nobody, including self-hosters on an old Postgres
build, still needs the tool. If that confirmation is not in the task you were given, stop
and ask.

## Why remove it

- It is the only user of `github.com/jackc/pgx/v5` and `database/sql`. Removing it drops
  that module and its indirect dependencies from `go.mod`, which leaves the AWS SDK as the
  only direct dependency. The open Dependabot PR #1 (a pgx bump) then has nothing to
  update and can be closed.
- It carries 428 lines and five of the nine lint findings from item 01, at 18% coverage.
- It calls `config.Load`, which constrains what the server can validate at startup (see
  item 05).

## Change

1. Delete `cmd/migrate/`.
2. `go mod tidy`. Confirm pgx and the `jackc` modules are gone from `go.mod` and `go.sum`.
3. Remove the "Migrating existing data from Supabase" section from `README.md` (or from
   `docs/operations.md` if item 10 has moved it), and any other mention of the tool. Add
   one line saying that the migration tool was last shipped in v1.0.0, for anyone who
   still needs it.
4. Remove `data.sql` and `schema.sql` from `.gitignore` if nothing else produces them.
5. If item 05 added a `ValidateServer()` split only to keep the tool working, fold it back
   into `Load`.

## Done when

- `grep -rn "pgx\|database/sql\|cmd/migrate" --include='*.go' --include='*.md' --include='go.mod' .`
  prints only the one-line note from step 3.
- `go build ./...` and the tests pass.
- Changeset (minor): the Postgres migration tool has been removed; use v1.0.0 to migrate.
