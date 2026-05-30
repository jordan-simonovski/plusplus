# plusplus

Slack karma service implemented in Go, designed as a long-running container.

## Features
- Listens to Slack Events API `message` and `app_mention` events and applies karma.
- Supports `+` and `-` runs with discord-karma parity rules (min 2 symbols, cap at 6 symbols => max delta 5).
- Slash commands:
  - `/leaderboard`
  - `/settings reply_mode thread|channel`
- DynamoDB storage with workspace (`team_id`) isolation.

## Environment Variables
- `PORT` (default: `8080`)
- `AWS_REGION` (default: `ap-southeast-2`)
- `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` (DynamoDB credentials; standard AWS SDK chain)
- `DYNAMODB_ENDPOINT` (optional; set to `http://localhost:8000` for DynamoDB Local, leave unset in prod)
- `DYNAMODB_KARMA_TABLE` (default: `plusplus_karma`)
- `DYNAMODB_SETTINGS_TABLE` (default: `plusplus_channel_settings`)
- `DYNAMODB_WORKSPACES_TABLE` (default: `plusplus_workspaces`)
- `MAX_KARMA_PER_ACTION` (default: `5`)
- `SLACK_SIGNING_SECRET` (required for real Slack traffic)
- `SLACK_BOT_TOKEN` (single-workspace / dev only; used to post via the Slack Web API when OAuth is **not** configured). When `SLACK_CLIENT_ID` is set (multi-tenant OAuth), per-workspace tokens are authoritative and this value is ignored.
- `PUBLIC_BASE_URL` (required when `SLACK_CLIENT_ID` is set; fixes the OAuth `redirect_uri` to a trusted origin instead of inferring it from request headers)

Tables are created automatically on startup (on-demand / pay-per-request billing) if they do not already exist. The IAM principal therefore needs `dynamodb:CreateTable` and `dynamodb:DescribeTable` in addition to the item operations (see deployment section).

## Local Development
### Prerequisites
- Go 1.24+
- Docker + Docker Compose

### Start local stack
```bash
make up
```

This starts:
- `dynamodb` (DynamoDB Local) on `localhost:8000` (in-memory)
- app on `localhost:8080` (tables auto-created on boot)

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
   - `channels:history`
   - `groups:history`
   - `chat:write`
   - `commands`
   - `usergroups:read`
   - `users:read`
3. Configure **Event Subscriptions**:
   - Request URL: `https://<public-url>/slack/events`
   - Subscribe to bot events:
     - `app_mention`
     - `message.channels`
     - `message.groups` (private channels)
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

## DynamoDB Deployment Notes
- Storage is DynamoDB. Three tables (karma, channel settings, workspaces) use **on-demand (pay-per-request)** billing, so there is no idle cost — you pay per read/write.
- In production the tables are provisioned by Terraform (see [Infrastructure](#infrastructure-terraform)). The app is **describe-first**: it verifies tables exist on boot and only creates them when missing (local dev / DynamoDB Local). So the production runtime credentials need `dynamodb:DescribeTable` but **not** `CreateTable`.
- The app authenticates with the standard AWS SDK credential chain. On Railway that means `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` / `AWS_REGION` env vars.
- Leave `DYNAMODB_ENDPOINT` unset in production (it is only for DynamoDB Local).

### Connecting to DynamoDB from outside AWS (Railway)
AWS's best practice is to avoid long-lived access keys and use temporary credentials. For a workload running outside AWS, the "correct" path is [IAM Roles Anywhere](https://docs.aws.amazon.com/rolesanywhere/latest/userguide/introduction.html) (X.509 certs from a private CA → short-lived STS credentials via a credential helper). It is the right answer at scale, but it requires standing up a private CA and running the credential helper inside the container — overkill for a single Slack bot.

The pragmatic, widely-used approach for Railway → DynamoDB is a **dedicated IAM user with a least-privilege policy and static keys** stored as Railway secrets. The AWS SDK picks these up automatically. If you later want temporary credentials, swap the IAM user for Roles Anywhere without touching application code (the SDK credential chain handles both).

## Railway + DynamoDB Setup

### 1) Create the runtime IAM user
The Terraform bootstrap config (see [Infrastructure](#infrastructure-terraform)) creates a least-privilege IAM user `plusplus-runtime` with exactly this policy — describe + item ops, scoped to the three tables and the leaderboard index:

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

Create an access key for that user (`aws iam create-access-key --user-name plusplus-runtime`) and rotate it periodically. There is no `CreateTable` because Terraform owns the tables.

### 2) Create Railway service
1. Create a new Railway project and add a service from this repository.
2. Use the default container entrypoint (no custom command needed with the `Dockerfile`).
3. Ensure Railway exposes port `8080`.

### 3) Configure Railway environment variables
Set these in Railway service variables:

- `PORT=8080`
- `AWS_REGION=<your region, e.g. ap-southeast-2>`
- `AWS_ACCESS_KEY_ID=<iam user access key id>`
- `AWS_SECRET_ACCESS_KEY=<iam user secret>`
- `MAX_KARMA_PER_ACTION=5`
- `SLACK_SIGNING_SECRET=<from slack app settings>`
- `SLACK_BOT_TOKEN=<xoxb token>`

For multi-workspace OAuth install (the `/slack/install` flow), also set:

- `SLACK_CLIENT_ID=<from slack app settings>`
- `SLACK_CLIENT_SECRET=<from slack app settings>`
- `TOKEN_ENCRYPTION_KEY=<base64 32-byte key>`
- `PUBLIC_BASE_URL=https://api.pluspluskarma.dev` — pins generated OAuth `redirect_uri`/install URLs to your domain. If unset, the app infers the base URL from the request `Host`. This value must exactly match a registered redirect URL in the Slack app manifest.

#### Custom domain (api.pluspluskarma.dev)
1. Railway → service → Settings → Networking → Custom Domain → add `api.pluspluskarma.dev`; Railway returns a CNAME target.
2. At your DNS provider for `pluspluskarma.dev`, add `CNAME api → <target>.up.railway.app`. Railway auto-provisions TLS once DNS resolves.
3. Set `PUBLIC_BASE_URL=https://api.pluspluskarma.dev` and redeploy.
4. Apply the updated `slack-app-manifest.yaml` so Slack's request/redirect URLs use the custom domain.

### Migrating existing data from Supabase
The one-time `cmd/migrate` tool copies the three tables (`karma_totals`, `channel_settings`, `slack_workspaces`) into DynamoDB. Bot-token ciphertext is copied as-is, so no encryption key is needed. It is idempotent (uses `PutItem`), so it is safe to re-run.

**From a Supabase SQL export (recommended — no Postgres needed).** Supabase's "Download backup" gives you a `schema.sql` and a `data.sql` (plain `pg_dump`). Point the tool at the data file; it parses the `COPY` blocks directly and ignores the `auth.*`/`storage.*` noise:

```bash
AWS_REGION=<region> AWS_ACCESS_KEY_ID=<id> AWS_SECRET_ACCESS_KEY=<secret> \
go run ./cmd/migrate data.sql
```

(Do not try to restore `schema.sql` into a vanilla Postgres — it references Supabase-only roles and extensions and will error.)

**From a live Postgres connection** (if the old database is still up):

```bash
DATABASE_URL=<old supabase connection string> \
AWS_REGION=<region> AWS_ACCESS_KEY_ID=<id> AWS_SECRET_ACCESS_KEY=<secret> \
go run ./cmd/migrate
```

Once cutover is verified, `cmd/migrate` and the `pgx` dependency can be deleted.

### 4) Configure Slack callback URLs
After Railway deploys and you have a public URL (custom domain `https://api.pluspluskarma.dev` or the default `https://<railway-domain>`):

- Event Subscriptions request URL: `https://api.pluspluskarma.dev/slack/events`
- `/leaderboard` command URL: `https://api.pluspluskarma.dev/slack/commands`
- `/settings` command URL: `https://api.pluspluskarma.dev/slack/commands`
- OAuth redirect URL: `https://api.pluspluskarma.dev/slack/oauth/callback`

### 5) Verify deployment
1. Hit `https://api.pluspluskarma.dev/healthz` and confirm `status: ok`.
2. In Slack, run `/leaderboard` and verify a response.
3. In a channel with the bot, send ambient events like `<@user> +++` and verify persisted karma.

## Infrastructure (Terraform)

DynamoDB and the CI/runtime IAM are managed in `terraform/`, applied from GitHub Actions via OIDC (no long-lived AWS keys in the repo).

Layout:
- `terraform/bootstrap/` — run **once, locally, with admin credentials**. Creates the S3 state bucket, the GitHub Actions OIDC provider, the CI role, and the `plusplus-runtime` IAM user. Uses local state on purpose (it creates the bucket the main config then uses).
- `terraform/` — the DynamoDB tables. Uses the S3 backend and is applied by CI.

The two are split so that **CI never has permission to modify its own IAM** — IAM is bootstrapped by a human, CI only manages tables.

### One-time bootstrap

```bash
cd terraform/bootstrap
terraform init
terraform apply \
  -var 'state_bucket_name=<globally-unique-bucket-name>' \
  -var 'aws_region=ap-southeast-2'
```

The GitHub OIDC provider is account-global (one per URL). If your account already has it, add `-var 'create_github_oidc_provider=false'` and the config will reference the existing one instead of creating a duplicate.

Note the outputs:
- `state_bucket_name` → put it in `terraform/backend.hcl` (replace `REPLACE_WITH_STATE_BUCKET_NAME`).
- `ci_role_arn` → add as a GitHub **repository variable** named `AWS_ROLE_ARN` (Settings → Secrets and variables → Actions → Variables).
- `runtime_user_name` → create its access key for Railway: `aws iam create-access-key --user-name plusplus-runtime`.

### CI

`.github/workflows/terraform.yml` runs on changes under `terraform/**`:
- **pull request** → `fmt -check`, `init`, `validate`, `plan`.
- **push to `main`** → the above plus `apply`.

The CI role's trust policy only accepts this repo's `main` branch and pull requests. To apply locally instead:

```bash
cd terraform
terraform init -backend-config=backend.hcl
terraform apply
```

> The `aws_dynamodb_table` resources intentionally use the deprecated `hash_key`/`range_key` arguments. The newer `key_schema` block triggers perpetual plan drift on GSIs (provider issue #46513); the deprecation warning is cosmetic and safe to ignore.
