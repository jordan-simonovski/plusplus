# 08. OAuth handler cleanup

All changes are in `internal/slack/oauth_handler.go` and its tests.

## Problems

1. **A code path that trusts request headers.** `inferPublicBaseURL` builds the OAuth
   redirect origin from `X-Forwarded-Host` and `X-Forwarded-Proto`. `baseURL` falls back
   to it when `redirectBaseURL` is empty. `config.Load` already refuses to enable OAuth
   without `PUBLIC_BASE_URL`, so the fallback is unreachable in the server, but it is
   still there for the next caller of `NewOAuthHandler` to trip over.
2. **Upstream errors echoed to the browser.** `Callback` writes `err.Error()` from
   `exchangeCode` into the response, and writes the raw `error` query parameter back as
   `"Slack OAuth error: " + errParam`.
3. **One secret, two jobs.** The OAuth `state` is signed with the Slack signing secret
   itself. The same key then authenticates Slack's requests and the app's own state
   tokens.
4. **Unbounded read.** `exchangeCode` uses `io.ReadAll` on the response.

## Change

1. Delete `inferPublicBaseURL` and the fallback in `baseURL`. `NewOAuthHandler` returns
   `(*OAuthHandler, error)` and fails when `redirectBaseURL` is empty. Update the call in
   `cmd/server/main.go` and the tests.
2. On exchange failure respond 502 with a fixed message ("Could not complete the Slack
   install. Please try again."). For the `error` query parameter respond 400 with a fixed
   message, plus the parameter only when it matches `^[a-z_]{1,40}$` (Slack's error codes
   look like `access_denied`).
   If item 04 has landed, log the real cause with `slog`. If it has not, use `log.Printf`.
3. Derive a separate state key once in `NewOAuthHandler`:
   `HMAC-SHA256(key = stateSecret, message = "plusplus-oauth-state-v1")`, and sign and
   verify state with that. States minted before a deploy stop verifying; they only live
   ten minutes and the user is sent back to install, so no migration is needed.
4. Wrap the response body in `io.LimitReader(resp.Body, 1<<20)`.

## Tests

- `NewOAuthHandler` with an empty base URL returns an error.
- A callback with `X-Forwarded-Host: evil.example` still redirects to the configured base
  URL.
- A failed exchange returns 502 and the body does not contain the upstream error string.
- `?error=<script>` returns 400 and the body does not contain `<script>`.
- A state signed with the raw secret (the old scheme) is rejected.
- The existing tests in `oauth_*_test.go` still pass.

## Out of scope

Single-use state (needs shared storage; the existing comment explains why it is skipped).
The state-less callback path for installs started on Slack's side, which is deliberate.

## Done when

- Tests above pass.
- `grep -rn "X-Forwarded" internal` prints nothing.
- No changeset (no behaviour change for a correctly configured deployment).
