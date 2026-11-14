# Backlog

Work identified in a codebase audit on 2026-11-14 against `main` at `3d7936e`. Read
`AGENTS.md` first. Each file here is a self-contained brief for one PR.

Code references name functions and files, not line numbers, because the code will move.
If a brief no longer matches the code, trust the code, and say what differed in the PR.

## Order

Do them one at a time, top to bottom, merging each before starting the next. That is
always safe. An item has landed when its file no longer exists in `docs/backlog/` on
`main`; where a brief says "if item N has landed", check for the file.

Every item from the 2026-11-14 audit has landed; there are no briefs here now. New work
can be added as numbered briefs in the same shape, with a row in a table here.

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
