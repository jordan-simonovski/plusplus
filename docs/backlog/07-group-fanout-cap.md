# 07. Cap user-group karma

Do after 06.

## Problem

`handleSubteamKarma` in `internal/slack/events_processor.go` applies karma to every member
of a mentioned user group, one after another, inside the request. There is no limit. For
a group of 500 people, one `@plusplus @everyone ++` message causes about 1,000 DynamoDB
writes and up to 500 `users.info` calls, takes far longer than Slack's three-second
window, and builds a reply of 500 lines, which is beyond what `chat.postMessage` accepts.
Anyone in the workspace can send that message, repeatedly.

The comment on the HTTP server in `cmd/server/main.go` ("no WriteTimeout; group karma
fan-out runs inside the request") exists because of this.

## Change

1. Add a constant `maxGroupMembers = 50` in `internal/slack`.
2. In `handleSubteamKarma`, after de-duplicating the member list, if it is longer than the
   limit: apply no karma, and post one message in the usual thread saying the group is too
   large for group karma, giving the limit and the group's size.
3. Put that sentence in `internal/domain/messages.go` with the other reply copy, as a
   function taking the size and the limit.

## Owner decision

The number. 50 is the default: at roughly 20 ms per member it stays within Slack's window
with room to spare, and a reply of 50 lines is still readable. Use 50, and state the
choice at the top of the PR description so the owner can change it.

## Tests

In `internal/slack/events_processor_test.go`, using the existing `fakeUserGroupLister`:

- 50 members: karma applied 50 times.
- 51 members: karma applied zero times, one message posted, containing "51" and "50".
- Duplicate IDs are removed before the count is taken.

## Out of scope

Making the limit configurable. Fanning out concurrently. Adding a write timeout to the
HTTP server.

## Done when

- Tests above pass.
- Changeset (patch): group karma now applies to user groups of up to 50 people; larger
  groups get a message instead.
