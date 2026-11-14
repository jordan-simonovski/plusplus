# plusplus

PlusPlus is a Slack app for giving your teammates karma. React to a message with 🔥 or
type `@plusplus @sam ++`, and PlusPlus keeps score per workspace, with a leaderboard and
a little snark. It's written in Go and runs as a single container backed by DynamoDB.

## Using it

- Applies karma from emoji reactions (`reaction_added` / `reaction_removed`): 👏 🙌 ⭐ 🔥 🏆 give 1–5, 🙄 👎 🤡 💩 🗑️ take 1–5. Each reactor's strongest reaction on a message counts. Reactions to a typed karma message (`@plusplus @sam ++`) go to the people it credited, not the poster. Channels and workspaces can remap tiers to their own emojis with `/plusplus-settings emoji`.
- Applies karma from `@user ++` text: via `app_mention` (`@plusplus @sam ++`) everywhere, and from plain channel messages (`message` events) when the app has the `*:history` scopes (self-hosted).
- Supports `+` and `-` runs with discord-karma parity rules (min 2 symbols, cap at 6 symbols => max delta 5).
- Slash commands:
  - `/plusplus-leaderboard`
  - `/plusplus-settings reply_mode thread|channel`
  - `/plusplus-settings emoji [workspace] [<tier> :emoji: | reset]`
  - Add `help` (or anything unknown) to either command for usage.
- DynamoDB storage with workspace (`team_id`) isolation.

## Add it to Slack

Install the hosted app from the
[Slack Marketplace](https://slack.com/marketplace/A0AS7F57GJH-plusplus-karma), or from
[pluspluskarma.dev](https://pluspluskarma.dev).

## Self-hosting

Want to run your own instance? Published multi-arch container images
(`ghcr.io/jordan-simonovski/plusplus`) are built on every push to `main`. See
[docs/self-hosting.md](docs/self-hosting.md) for the full operator guide:
required environment variables, the image name and tags, and DynamoDB table
setup (let the app create them, or pre-create with least-privilege IAM).

### Environment variables
- `PORT` (default: `8080`)
- `AWS_REGION` (default: `ap-southeast-2`)
- `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` (DynamoDB credentials; standard AWS SDK chain)
- `DYNAMODB_ENDPOINT` (optional; set to `http://localhost:8000` for DynamoDB Local, leave unset in prod)
- `DYNAMODB_KARMA_TABLE` (default: `plusplus_karma`)
- `DYNAMODB_SETTINGS_TABLE` (default: `plusplus_channel_settings`)
- `DYNAMODB_WORKSPACES_TABLE` (default: `plusplus_workspaces`)
- `MAX_KARMA_PER_ACTION` (default: `5`; a whole number of at least 1)
- `SLACK_SIGNING_SECRET` (required; the server will not start without it)
- `SLACK_BOT_TOKEN` (required unless `SLACK_CLIENT_ID` is set; single-workspace / dev only; used to post via the Slack Web API when OAuth is **not** configured). When `SLACK_CLIENT_ID` is set (multi-tenant OAuth), per-workspace tokens are authoritative and this value is ignored.
- `PUBLIC_BASE_URL` (required when `SLACK_CLIENT_ID` is set; fixes the OAuth `redirect_uri` to a trusted origin instead of inferring it from request headers)
- `SLACK_OAUTH_SCOPES` (optional; bot scopes `/slack/install` requests. Defaults to the Marketplace set without `*:history`. Self-hosted installs that want plain `@sam ++` add `channels:history,groups:history`.)

Tables are created automatically on startup (on-demand / pay-per-request billing) if they do not already exist. The IAM principal therefore needs `dynamodb:CreateTable` and `dynamodb:DescribeTable` in addition to the item operations (see deployment section).

## Contribute

Contributions are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for how to run it,
the checks a pull request must pass, and when to add a changeset. Coding agents should
start at [AGENTS.md](AGENTS.md).

### Run it locally
#### Prerequisites
- Go (the version in `go.mod`)
- Docker + Docker Compose

#### Start local stack
```bash
make up
```

This starts:
- `dynamodb` (DynamoDB Local) on `localhost:8000` (in-memory)
- app on `localhost:8080` (tables auto-created on boot)

#### Stop local stack
```bash
make down
```

#### Run service directly
```bash
make run
```

#### Dev commands
```bash
make check            # format check, vet, lint and race tests (what CI runs)
make fmt
make test-integration
```

### Slack app for development
1. Create a Slack app **from a manifest** using [`slack-app-manifest.self-hosted.yaml`](slack-app-manifest.self-hosted.yaml) (replace `<your-host>` / `<public-url>`), then install it to your workspace. It includes reactions plus the `*:history` scopes for plain `@sam ++`. Drop `channels:history`, `groups:history` and the `message.*` events if you only want reactions and `@plusplus` mentions. (The Marketplace app uses [`slack-app-manifest.yaml`](slack-app-manifest.yaml).)
2. Set local env vars:
   - `SLACK_SIGNING_SECRET`
   - `SLACK_BOT_TOKEN`

For local callbacks, use a tunnel (for example `ngrok`):
```bash
ngrok http 8080
```

## Operations

How the hosted app is deployed (DynamoDB, Railway, Terraform and its CI) is in
[docs/operations.md](docs/operations.md).

## Licence

MIT; see [LICENSE.md](LICENSE.md).
