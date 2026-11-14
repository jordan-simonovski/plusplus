# Operating the hosted PlusPlus

How the hosted Marketplace app is deployed: DynamoDB, Railway and the Terraform that
manages them. To run your own instance, see [self-hosting.md](self-hosting.md) instead.

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
- `PUBLIC_BASE_URL=https://api.pluspluskarma.dev` — pins generated OAuth `redirect_uri`/install URLs to your domain. Required whenever `SLACK_CLIENT_ID` is set; the server will not start without it. This value must exactly match a registered redirect URL in the Slack app manifest.

#### Custom domain (api.pluspluskarma.dev)
1. Railway → service → Settings → Networking → Custom Domain → add `api.pluspluskarma.dev`; Railway returns a CNAME target.
2. At your DNS provider for `pluspluskarma.dev`, add `CNAME api → <target>.up.railway.app`. Railway auto-provisions TLS once DNS resolves.
3. Set `PUBLIC_BASE_URL=https://api.pluspluskarma.dev` and redeploy.
4. Apply the updated `slack-app-manifest.yaml` so Slack's request/redirect URLs use the custom domain.

### 4) Configure Slack callback URLs
After Railway deploys and you have a public URL (custom domain `https://api.pluspluskarma.dev` or the default `https://<railway-domain>`):

- Event Subscriptions request URL: `https://api.pluspluskarma.dev/slack/events`
- `/plusplus-leaderboard` command URL: `https://api.pluspluskarma.dev/slack/commands`
- `/plusplus-settings` command URL: `https://api.pluspluskarma.dev/slack/commands`
- OAuth redirect URL: `https://api.pluspluskarma.dev/slack/oauth/callback`

### 5) Verify deployment
1. Hit `https://api.pluspluskarma.dev/healthz` and confirm `status: ok`.
2. In Slack, run `/plusplus-leaderboard` and verify a response.
3. In a channel with the bot, send ambient events like `<@user> +++` and verify persisted karma.

## Infrastructure (Terraform)

DynamoDB and the CI/runtime IAM are managed in `terraform/`, applied from GitHub Actions via OIDC (no long-lived AWS keys in the repo).

Layout:
- `terraform/bootstrap/` — run **once, locally, with admin credentials**. Creates the S3 state bucket, the GitHub Actions OIDC provider, the CI role, and the `plusplus-runtime` IAM user. Uses local state on purpose (it creates the bucket the main config then uses).
- `terraform/` — the DynamoDB tables. Uses the S3 backend and is applied by CI.

There are two deployments with separate data: the Marketplace app (`plusplus_*` tables, `plusplus-runtime` user) and the self-hosted app (`plusplus_sh_*` tables, `plusplus-selfhosted-runtime` user). Each runtime user can only reach its own tables. When adding tables, apply `terraform/bootstrap` first so the CI role is allowed to create them.

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
- `ci_role_arn` → add as a GitHub **repository variable** named `AWS_ROLE_ARN` (Settings → Secrets and variables → Actions → Variables). Only `main` can assume it.
- `ci_plan_role_arn` → add as a repository variable named `AWS_PLAN_ROLE_ARN`. Pull requests use this read-only role for `plan`.
- `runtime_user_name` → create its access key for Railway: `aws iam create-access-key --user-name plusplus-runtime`.

### CI

`.github/workflows/terraform.yml` runs on changes under `terraform/**`:
- **pull request** → `fmt -check`, `init`, `validate`, `plan`.
- **push to `main`** → the above plus `apply`.

The apply role's trust policy only accepts this repo's `main` branch; pull requests get the read-only plan role. To apply locally instead:

```bash
cd terraform
terraform init -backend-config=backend.hcl
terraform apply
```

> The `aws_dynamodb_table` resources intentionally use the deprecated `hash_key`/`range_key` arguments. The newer `key_schema` block triggers perpetual plan drift on GSIs (provider issue #46513); the deprecation warning is cosmetic and safe to ignore.
