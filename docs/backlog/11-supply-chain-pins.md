# 11. Dependabot and pinned actions/images

## Problem

- Workflows reference actions by mutable tag (`actions/checkout@v4`,
  `docker/build-push-action@v6`, `aws-actions/configure-aws-credentials@v4`, and others).
  `docker.yml` has `packages: write` and `terraform.yml` assumes an AWS role that applies
  to production, so a moved tag on any of those actions runs with real privileges.
- The `Dockerfile` uses `golang:1.26.11-alpine` and
  `gcr.io/distroless/static-debian12:nonroot` by tag. `amazon/dynamodb-local` is pinned
  to a version tag in `docker-compose.yml` and the `integration` job in
  `.github/workflows/go.yml`, but not by digest.
- There is no `.github/dependabot.yml`. Dependabot security updates are evidently on (PR
  #1 is one), but nothing proposes routine updates for Go modules, actions, npm or images.

- GitHub lists eight open Dependabot alerts on `main` for `js-yaml` in
  `package-lock.json` (six high, two medium; checked 2026-11-14). `js-yaml` is a
  transitive dependency of the changeset tooling and is not in the shipped binary. (The
  pgx alert was fixed when Go CI was added.)

## Change

0. Clear the `js-yaml` alerts: run `npm audit fix`, confirm `npm audit` reports nothing
   high, and confirm `npx changeset status` and `node landing/scripts/build-changelog.mjs`
   still run. List the alerts that remain open, if any, in the PR description.

1. Add `.github/dependabot.yml` with weekly updates for four ecosystems: `gomod` at `/`,
   `github-actions` at `/`, `npm` at `/`, and `docker` at `/`. Group minor and patch
   updates per ecosystem so each produces one PR a week. Group the AWS SDK modules
   together (`github.com/aws/*`).
2. Pin every `uses:` in `.github/workflows/*.yml` to a full commit SHA with the version in
   a trailing comment, for example `uses: actions/checkout@<sha> # v4.2.2`. Resolve each
   SHA from the action's own repository at its current release with
   `gh api repos/<owner>/<repo>/git/ref/tags/<tag>` (if the result is an annotated tag,
   follow it to the commit). Do not write SHAs from memory.
3. Pin both `Dockerfile` base images by digest, keeping the tag for readability:
   `golang:1.26.11-alpine@sha256:…`. Get digests with `docker buildx imagetools inspect`.
   If Docker is not available, do not guess digests: leave the `Dockerfile` as it is, say
   so in the PR description, and do the rest.
4. Pin `amazon/dynamodb-local` by digest as well as tag, in both `docker-compose.yml` and
   the `integration` job's service container in `.github/workflows/go.yml`, keeping the two
   identical.

## Out of scope

Image signing, SBOMs, provenance attestations. Changing what the workflows do.

## Done when

- `grep -rnE "uses: [^ ]+@v[0-9]" .github/workflows` prints nothing.
- `docker build .` succeeds with the pinned digests.
- All workflows are green on the PR. `terraform.yml` and `docker.yml` only run fully on
  `main`, so state in the PR which jobs could not be exercised before merge.
- No changeset.
