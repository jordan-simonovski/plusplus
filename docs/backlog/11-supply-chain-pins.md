# 11. Dependabot and pinned actions/images

## Problem

- Workflows reference actions by mutable tag (`actions/checkout@v4`,
  `docker/build-push-action@v6`, `aws-actions/configure-aws-credentials@v4`, and others).
  `docker.yml` has `packages: write` and `terraform.yml` assumes an AWS role that applies
  to production, so a moved tag on any of those actions runs with real privileges.
- The `Dockerfile` uses `golang:1.26.1-alpine` and
  `gcr.io/distroless/static-debian12:nonroot` by tag. `docker-compose.yml` uses
  `amazon/dynamodb-local:latest`.
- There is no `.github/dependabot.yml`. Dependabot security updates are evidently on (PR
  #1 is one), but nothing proposes routine updates for Go modules, actions, npm or images.

## Change

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
   `golang:1.26.1-alpine@sha256:…`. Get digests with `docker buildx imagetools inspect`.
   If Docker is not available, do not guess digests: leave the `Dockerfile` as it is, say
   so in the PR description, and do the rest.
4. Replace `amazon/dynamodb-local:latest` in `docker-compose.yml` with the current version
   tag, and use the same tag for the service container in `.github/workflows/go.yml` if
   that job exists.

## Out of scope

Image signing, SBOMs, provenance attestations. Changing what the workflows do.

## Done when

- `grep -rnE "uses: [^ ]+@v[0-9]" .github/workflows` prints nothing.
- `docker build .` succeeds with the pinned digests.
- All workflows are green on the PR. `terraform.yml` and `docker.yml` only run fully on
  `main`, so state in the PR which jobs could not be exercised before merge.
- No changeset.
