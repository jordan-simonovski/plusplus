# 05. Validate config at startup

## Problem

`config.Load` in `internal/config/config.go` accepts settings that produce a server that
starts and then fails every request:

- `SLACK_SIGNING_SECRET` unset: the server starts, and `VerifyRequestSignature` rejects
  every request with 401. Nothing at startup says why.
- OAuth off (`SLACK_CLIENT_ID` unset) and `SLACK_BOT_TOKEN` unset: karma is applied but
  every reply fails with "no bot token for workspace".
- `MAX_KARMA_PER_ACTION=abc`: `getenvIntDefault` silently uses 5.
- `SLACK_APP_TOKEN` is read into `Config.SlackAppToken` and used nowhere.

The package has no tests (0% coverage).

## Change

1. `Load` returns an error when `SLACK_SIGNING_SECRET` is empty.
2. `Load` returns an error when `SLACK_CLIENT_ID` and `SLACK_BOT_TOKEN` are both empty,
   saying that one of the two modes must be configured.
3. A non-numeric `MAX_KARMA_PER_ACTION` is an error that names the variable. An unset one
   still defaults to 5.
4. Remove `SlackAppToken` and the `SLACK_APP_TOKEN` read. Remove the variable from
   `README.md` and `docs/self-hosting.md` if it is mentioned.
5. `cmd/migrate` also calls `config.Load` and has no Slack settings. Check how it uses the
   config and keep it working: move checks 1 and 2 into a `ValidateServer()` method on
   `Config` that only `cmd/server/main.go` calls, if that is what it takes.
6. Add `internal/config/config_test.go`: a table test using `t.Setenv` covering each error
   above, the defaults, the OAuth requirements that already exist (client secret, public
   base URL, encryption key), and one valid configuration for each mode.

`make run` and `docker-compose.yml` already set a dev signing secret and bot token, so
local development is unaffected. Confirm that.

## Out of scope

New configuration options. Reading config from files.

## Done when

- The table test passes and `internal/config` coverage is above 80%.
- `make run` still starts.
- Changeset (patch): the server now refuses to start with a clear message when Slack
  credentials are missing or `MAX_KARMA_PER_ACTION` is not a number. This can stop a
  misconfigured self-hosted deployment that previously started, so say so.
