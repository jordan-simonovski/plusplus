# 14. Collapse and test the Slack Web API client

## Problem

`internal/slack/client.go` has three methods (`PostMessage`, `ListUserGroupMembers`,
`usersInfo`) that each repeat the same routine: build a request, set the bearer token,
send, read the whole body, decode JSON, check `ok`. The file has no tests. Three details
in that routine are wrong in all three copies:

- The HTTP status is never checked. A 429 or a 5xx with a non-JSON body surfaces as
  "decode slack response" with the real cause lost.
- The body is read with `io.ReadAll` and no limit.
- `TeamResolvingClient.apiClient` in `team_client.go` builds a new `APIClient`, and with
  it a new `http.Client`, for every call.

## Change

1. Add one unexported method on `APIClient`:
   `call(ctx, method string, contentType string, body io.Reader, out any) error`, where
   `method` is the Slack method name such as `chat.postMessage`. It:
   - posts to `https://slack.com/api/` plus the method name with the bearer token;
   - returns an error naming the method and status when the status is not 2xx, including
     the `Retry-After` header value when the status is 429;
   - decodes at most 1 MiB of the body into `out`.
2. Every response type embeds a small struct with `OK bool` and `Error string`, and `call`
   returns `fmt.Errorf("slack %s failed: %s", method, errCode)` when `ok` is false. The
   three public methods shrink to building the payload and calling `call`.
3. Give `TeamResolvingClient` one shared `*http.Client` (10 second timeout, as today) and
   pass it to each `APIClient` instead of creating one per call.
4. Remove the three URL constants.

Error strings are matched by nothing in the codebase, but keep "slack <method> failed:
<code>" recognisable for operators reading logs.

## Tests

Add `internal/slack/client_test.go`, stubbing the transport with `roundTripFunc` from
`oauth_callback_test.go`:

- `PostMessage` sends the bearer token, JSON content type, channel, text, and `thread_ts`
  only when set.
- `ok: false` returns an error containing the method name and Slack's error code.
- A 429 with `Retry-After: 30` returns an error mentioning 429 and 30.
- A 500 with an HTML body returns an error mentioning the status, not a JSON decode error.
- `DisplayName` prefers display name, then profile real name, then real name, and errors
  when all are empty.
- `IsWorkspaceAdmin` is true for an admin and for an owner.

## Out of scope

Retrying, rate limiting, pagination, new API methods.

## Done when

- Tests above pass; `internal/slack/client.go` is covered above 85%.
- `grep -c "httpClient.Do" internal/slack/client.go` prints 1.
- No changeset.
