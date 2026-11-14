# 17. Bound the name cache

## Problem

`CachedNameResolver` in `internal/slack/name_cache.go` keeps one entry per (workspace,
user) it has ever looked up. Entries have a one-hour expiry but are only replaced when the
same user is looked up again; nothing removes them. The hosted app serves every installed
workspace from one process, so the map grows with the total number of users seen since
the last restart.

## Change

1. Add a `maxEntries` field, set to 10,000 in `NewCachedNameResolver`.
2. When storing a new entry would exceed the limit, first delete every expired entry. If
   the map is still at the limit, clear it.
3. Mark it:
   `// ponytail: clears the whole cache when full of live entries; switch to LRU if lookups after a clear ever show up in Slack rate limits.`

Do not add a dependency, a background goroutine or a configuration option.

## Tests

In `name_cache_test.go`, using the existing fake clock (`now` field) and a small
`maxEntries` set directly on the struct:

- Filling past the limit with expired entries leaves only live ones.
- Filling past the limit with live entries leaves the map at or below the limit, and the
  newest entry is present.
- Existing tests pass unchanged.

## Done when

- Tests above pass under `-race`.
- No changeset.
