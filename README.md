# plusplus

Slack karma service implemented in Go, designed as a long-running container.

## Features
- Listens to Slack Events API `app_mention` events and applies karma.
- Supports `+` and `-` runs with discord-karma parity rules (min 2 symbols, cap at 6 symbols => max delta 5).
- Slash commands:
  - `/leaderboard`
  - `/settings reply_mode thread|channel`
- Postgres storage (Supabase-compatible) with workspace (`team_id`) isolation.

## Environment Variables
- `PORT` (default: `8080`)
- `DATABASE_URL` (default local: `postgres://postgres:postgres@localhost:5432/plusplus?sslmode=disable`)
- `MAX_KARMA_PER_ACTION` (default: `5`)
- `SLACK_SIGNING_SECRET` (required for real Slack traffic)
- `SLACK_BOT_TOKEN` (required to post messages via Slack Web API)

## Local Development
### Prerequisites
- Go 1.24+
- Docker + Docker Compose

### Start local stack
```bash
make up
```

This starts:
- `postgres` on `localhost:5432` (schema auto-initialized)
- app on `localhost:8080`

### Stop local stack
```bash
make down
```

### Run service directly
```bash
make run
```

### Dev commands
```bash
make fmt
make lint
make test
make test-integration
```

## Slack App Setup (dev)
1. Create a Slack app and install it to your workspace.
2. Add bot token scopes:
   - `app_mentions:read`
   - `chat:write`
   - `commands`
3. Configure **Event Subscriptions**:
   - Request URL: `https://<public-url>/slack/events`
   - Subscribe to bot event: `app_mention`
4. Configure Slash Commands:
   - `/leaderboard` Request URL: `https://<public-url>/slack/commands`
   - `/settings` Request URL: `https://<public-url>/slack/commands`
5. Set local env vars:
   - `SLACK_SIGNING_SECRET`
   - `SLACK_BOT_TOKEN`

For local callbacks, use a tunnel (for example `ngrok`):
```bash
ngrok http 8080
```

## Supabase Deployment Notes
- Set `DATABASE_URL` to the Supabase Postgres connection string.
- Keep `sslmode=require` for hosted Supabase.
- App startup now runs SQL migrations automatically from `internal/persistence/migrations/*.sql`.
- Existing schema in `scripts/init-postgres.sql` matches migration `001_init.sql`.

## Railway + Supabase Setup

### 1) Create Supabase project
1. Create a new Supabase project in your preferred region.
2. Open the SQL editor and run the contents of `scripts/init-postgres.sql`.
3. Copy the connection string from Supabase and ensure it includes `sslmode=require`.

Example:
```text
postgres://postgres.<project-ref>:<password>@aws-0-<region>.pooler.supabase.com:6543/postgres?sslmode=require
```

### 2) Create Railway service
1. Create a new Railway project and add a service from this repository.
2. Set the service start command to the default container entrypoint (no custom command needed when using `Dockerfile`).
3. Ensure Railway exposes port `8080`.

### 3) Configure Railway environment variables
Set these in Railway service variables:

- `PORT=8080`
- `DATABASE_URL=<your supabase connection string>`
- `MAX_KARMA_PER_ACTION=5`
- `SLACK_SIGNING_SECRET=<from slack app settings>`
- `SLACK_BOT_TOKEN=<xoxb token>`

### 4) Configure Slack callback URLs
After Railway deploys and gives you a public URL:

- Event Subscriptions request URL: `https://<railway-domain>/slack/events`
- `/leaderboard` command URL: `https://<railway-domain>/slack/commands`
- `/settings` command URL: `https://<railway-domain>/slack/commands`

### 5) Verify deployment
1. Hit `https://<railway-domain>/healthz` and confirm `status: ok`.
2. In Slack, run `/leaderboard` and verify a response.
3. In a channel with the bot, send mention events like `<@user> +++` and verify persisted karma.
