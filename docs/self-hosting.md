# Self-hosting plusplus

plusplus ships as a single static Go binary in a distroless container. To run
your own instance you need three things:

1. The published container image.
2. A DynamoDB backend (real AWS DynamoDB in production, or DynamoDB Local for
   testing) plus credentials to reach it.
3. A Slack app and the environment variables that wire it together.

The container listens on `PORT` (default `8080`) and exposes `GET /healthz` for
health checks. It runs as a non-root user and needs no writable volumes.

---

## 1. Container image

Images are published to GitHub Container Registry on every push to `main`:

```
ghcr.io/jordan-simonovski/plusplus
```

Tags:

| Tag         | Meaning                                                        |
| ----------- | -------------------------------------------------------------- |
| `latest`    | Most recent build from `main`.                                 |
| `X.Y.Z`     | The release build of that version (e.g. `1.0.0`). Never moved. |
| `sha-<sha>` | The exact commit, for reproducible pins.                       |

Pin to a version tag in production rather than `latest`:

```bash
docker pull ghcr.io/jordan-simonovski/plusplus:1.0.0
```

The image is multi-arch (`linux/amd64` and `linux/arm64`); Docker pulls the
variant matching your host automatically.

---

## 2. Environment variables

All configuration is via environment variables. Defaults come from
`internal/config/config.go`.

### Core

| Variable               | Required | Default                      | Notes                                                              |
| ---------------------- | -------- | ---------------------------- | ------------------------------------------------------------------ |
| `PORT`                 | no       | `8080`                       | HTTP listen port (1–65535).                                        |
| `MAX_KARMA_PER_ACTION` | no       | `5`                          | Max karma delta per message. Must be ≥ 1.                          |
| `SLACK_SIGNING_SECRET` | **yes**  | —                            | From the Slack app **Basic Information** page. Verifies requests.  |

### DynamoDB / AWS

| Variable                    | Required | Default                     | Notes                                                                       |
| --------------------------- | -------- | --------------------------- | --------------------------------------------------------------------------- |
| `AWS_REGION`                | no       | `ap-southeast-2`            | Region your tables live in.                                                 |
| `AWS_ACCESS_KEY_ID`         | yes\*    | —                           | Standard AWS SDK chain. Omit if using an IAM role / instance profile.       |
| `AWS_SECRET_ACCESS_KEY`     | yes\*    | —                           | As above.                                                                   |
| `DYNAMODB_ENDPOINT`         | no       | — (real AWS)                | Set to e.g. `http://dynamodb:8000` for DynamoDB Local. **Leave unset in production.** |
| `DYNAMODB_KARMA_TABLE`      | no       | `plusplus_karma`            | Override table name.                                                        |
| `DYNAMODB_SETTINGS_TABLE`   | no       | `plusplus_channel_settings` | Override table name.                                                        |
| `DYNAMODB_WORKSPACES_TABLE` | no       | `plusplus_workspaces`       | Override table name.                                                        |

\* Credentials are required unless the runtime provides them another way (an
EC2/ECS/EKS IAM role, or any other source in the AWS SDK default credential
chain). When `DYNAMODB_ENDPOINT` is set and no key is present, dummy `local`
credentials are injected for DynamoDB Local.

### Slack — single-workspace mode (simplest)

Run the bot for one workspace using a static bot token. Set this **instead of**
the OAuth variables below.

| Variable          | Required | Notes                                                          |
| ----------------- | -------- | -------------------------------------------------------------- |
| `SLACK_BOT_TOKEN` | **yes**  | `xoxb-…` token from **OAuth & Permissions**. Posts to Slack.   |

### Slack — multi-workspace OAuth mode

Enable the `/slack/install` flow so others can install your instance. Setting
`SLACK_CLIENT_ID` switches the app into this mode; `SLACK_BOT_TOKEN` is then
**ignored** (per-workspace tokens are stored, encrypted, in DynamoDB).

| Variable               | Required (in this mode) | Notes                                                                                      |
| ---------------------- | ----------------------- | ------------------------------------------------------------------------------------------ |
| `SLACK_CLIENT_ID`      | **yes**                 | From the Slack app **Basic Information** page. Presence enables OAuth mode.                 |
| `SLACK_CLIENT_SECRET`  | **yes**                 | From the same page. Validated at startup.                                                  |
| `PUBLIC_BASE_URL`      | **yes**                 | e.g. `https://karma.example.com`. Pins the OAuth `redirect_uri` to a trusted origin. Must exactly match the redirect URL registered in the Slack app. |
| `TOKEN_ENCRYPTION_KEY` | **yes**                 | Base64-encoded 32-byte AES key; encrypts stored bot tokens. Generate with `openssl rand -base64 32`. |
| `SLACK_OAUTH_SCOPES`   | no                      | Comma-separated bot scopes `/slack/install` requests. Defaults to the Marketplace set (no `*:history`); add `channels:history,groups:history` for plain `@someone ++`. |

The app **fails to start** if `SLACK_CLIENT_ID` is set without
`SLACK_CLIENT_SECRET`, `PUBLIC_BASE_URL`, or a valid `TOKEN_ENCRYPTION_KEY`.

---

## 3. DynamoDB setup

The app stores everything in three DynamoDB tables, all **on-demand
(pay-per-request)** so there is no idle cost. Workspaces are isolated by
`team_id`.

### Schema

| Table (default name)        | Partition key (HASH) | Sort key (RANGE) | Indexes                                                       |
| --------------------------- | -------------------- | ---------------- | ------------------------------------------------------------- |
| `plusplus_karma`            | `team_id` (S)        | `user_id` (S)    | GSI `leaderboard`: HASH `team_id` (S), RANGE `karma_total` (N), projection ALL |
| `plusplus_channel_settings` | `team_id` (S)        | `channel_id` (S) | none                                                          |
| `plusplus_workspaces`       | `team_id` (S)        | —                | none                                                          |

You only need to declare key/index attributes; DynamoDB is schemaless for the
rest (`karma_total`, `karma_max`, `last_activity_at`, `reply_mode`,
`snark_level`, `bot_token_ciphertext`, etc. are written as needed).

### Option A — let the app create the tables (easiest)

On startup the app is **describe-first**: it checks each table and creates it
only if missing, then waits for it to become `ACTIVE`. This is the path used by
DynamoDB Local and works against real AWS too.

For this, the credentials need:

- `dynamodb:DescribeTable`
- `dynamodb:CreateTable`
- `dynamodb:GetItem`, `dynamodb:PutItem`, `dynamodb:UpdateItem`, `dynamodb:Query`

### Option B — pre-create the tables (least privilege at runtime)

Create the tables yourself (Terraform, console, or the CLI below), then the
runtime credentials only need describe + item operations — **no
`CreateTable`**. This is the recommended production posture.

Minimal runtime IAM policy (replace `REGION`/`ACCOUNT_ID`, and table names if
you overrode them):

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "DescribeTables",
      "Effect": "Allow",
      "Action": ["dynamodb:DescribeTable"],
      "Resource": [
        "arn:aws:dynamodb:REGION:ACCOUNT_ID:table/plusplus_karma",
        "arn:aws:dynamodb:REGION:ACCOUNT_ID:table/plusplus_channel_settings",
        "arn:aws:dynamodb:REGION:ACCOUNT_ID:table/plusplus_workspaces"
      ]
    },
    {
      "Sid": "ItemOps",
      "Effect": "Allow",
      "Action": ["dynamodb:GetItem", "dynamodb:PutItem", "dynamodb:UpdateItem", "dynamodb:Query"],
      "Resource": [
        "arn:aws:dynamodb:REGION:ACCOUNT_ID:table/plusplus_karma",
        "arn:aws:dynamodb:REGION:ACCOUNT_ID:table/plusplus_karma/index/*",
        "arn:aws:dynamodb:REGION:ACCOUNT_ID:table/plusplus_channel_settings",
        "arn:aws:dynamodb:REGION:ACCOUNT_ID:table/plusplus_workspaces"
      ]
    }
  ]
}
```

Create the tables with the AWS CLI:

```bash
aws dynamodb create-table \
  --table-name plusplus_karma \
  --billing-mode PAY_PER_REQUEST \
  --attribute-definitions \
    AttributeName=team_id,AttributeType=S \
    AttributeName=user_id,AttributeType=S \
    AttributeName=karma_total,AttributeType=N \
  --key-schema \
    AttributeName=team_id,KeyType=HASH \
    AttributeName=user_id,KeyType=RANGE \
  --global-secondary-indexes \
    'IndexName=leaderboard,KeySchema=[{AttributeName=team_id,KeyType=HASH},{AttributeName=karma_total,KeyType=RANGE}],Projection={ProjectionType=ALL}'

aws dynamodb create-table \
  --table-name plusplus_channel_settings \
  --billing-mode PAY_PER_REQUEST \
  --attribute-definitions \
    AttributeName=team_id,AttributeType=S \
    AttributeName=channel_id,AttributeType=S \
  --key-schema \
    AttributeName=team_id,KeyType=HASH \
    AttributeName=channel_id,KeyType=RANGE

aws dynamodb create-table \
  --table-name plusplus_workspaces \
  --billing-mode PAY_PER_REQUEST \
  --attribute-definitions AttributeName=team_id,AttributeType=S \
  --key-schema AttributeName=team_id,KeyType=HASH
```

If you prefer infrastructure-as-code, `terraform/dynamodb.tf` in this repo
defines the same tables and is a good starting point.

---

## 4. Run it

### Docker (single workspace)

```bash
docker run -d --name plusplus -p 8080:8080 \
  -e AWS_REGION=us-east-1 \
  -e AWS_ACCESS_KEY_ID=... \
  -e AWS_SECRET_ACCESS_KEY=... \
  -e SLACK_SIGNING_SECRET=... \
  -e SLACK_BOT_TOKEN=xoxb-... \
  ghcr.io/jordan-simonovski/plusplus:0.1.0
```

### docker compose (with DynamoDB Local, for testing)

The repo's `docker-compose.yml` runs the app against DynamoDB Local. To use the
published image instead of building locally, point the `app` service at
`ghcr.io/jordan-simonovski/plusplus:latest`.

### Behind a reverse proxy

Slack calls your instance over HTTPS, so terminate TLS at a proxy (nginx,
Caddy, Traefik, a cloud load balancer) and forward to the container's `PORT`.
In OAuth mode, set `PUBLIC_BASE_URL` to the externally visible HTTPS origin.

---

## 5. Wire up Slack

Point your Slack app at your instance's public URL:

- Event Subscriptions request URL: `https://<your-host>/slack/events`
- `/plusplus-leaderboard` command URL: `https://<your-host>/slack/commands`
- `/plusplus-settings` command URL: `https://<your-host>/slack/commands`
- OAuth redirect URL (OAuth mode only): `https://<your-host>/slack/oauth/callback`

The quickest setup is to create the app from
[`slack-app-manifest.self-hosted.yaml`](../slack-app-manifest.self-hosted.yaml)
(replace `<your-host>`). Bot scopes: `app_mentions:read`, `channels:history`,
`groups:history`, `chat:write`, `commands`, `reactions:read`, `usergroups:read`,
`users:read`; events: `app_mention`, `message.channels`, `message.groups`,
`reaction_added`, `reaction_removed`.

- `channels:history` / `groups:history` + `message.*` events give plain `@someone ++`
  karma. Leave them out for reactions and `@plusplus @someone ++` only. Slack only
  sends a bot the messages from channels it's a member of, so plain `@someone ++`
  does nothing until someone runs `/invite @plusplus` in that channel.
- `reactions:read` + `reaction_*` events give emoji reaction karma.
- In OAuth mode, set `SLACK_OAUTH_SCOPES` to the same comma-separated scope list.
  It defaults to the Marketplace set, which has no `*:history` scopes.

See the main [README](../README.md#slack-app-setup-dev) for the full Slack app walkthrough.

---

## Upgrading to reaction karma

Reaction karma needs a new scope and two new events, and the default OAuth scope
list no longer includes the history scopes. When you upgrade an existing instance:

1. In your Slack app, add the `reactions:read` scope and subscribe to the
   `reaction_added` and `reaction_removed` bot events (or re-apply
   [`slack-app-manifest.self-hosted.yaml`](../slack-app-manifest.self-hosted.yaml)).
2. **OAuth mode:** set `SLACK_OAUTH_SCOPES` to your full scope list. Without it,
   new installs request the Marketplace set, which has no `*:history` scopes, so
   plain `@someone ++` stops working for them. The server logs the scopes it
   requests at startup.
3. Reinstall the app in each workspace (or re-run `/slack/install`) so the bot
   token picks up `reactions:read`. Until then, reactions do nothing.

Plain `@someone ++` keeps working for existing installs that still have the
history scopes and `message.*` events.

---

## 6. Verify

1. `curl https://<your-host>/healthz` → `{"status":"ok",...}`.
2. Run `/plusplus-leaderboard` in Slack and confirm a response.
3. Invite the bot to a channel (`/invite @plusplus`), send `@someone +++` there and
   confirm karma persists. If nothing happens, the bot probably isn't in the channel.
4. React to someone's message with 🔥 and confirm a threaded "gained 4 karma" reply.
