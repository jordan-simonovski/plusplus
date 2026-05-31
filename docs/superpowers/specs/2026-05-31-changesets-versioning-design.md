# Semantic Versioning via Changesets — Design

Date: 2026-05-31
Status: Approved (pending spec review)

## Goal

Add semantic versioning to PlusPlus (a Go app) using [Changesets](https://github.com/changesets/changesets).
`package.json` becomes the single source of truth for the version. The version is:

1. Baked into the Go binary at build time and shown in the Slack `/settings` output.
2. Shown in the landing page top nav.
3. Backed by a dedicated landing changelog page that renders `CHANGELOG.md`.

Releases are cut **locally** by the maintainer. CI never bumps the version or pushes.

## Constraints / decisions

- Package manager: **npm**.
- Version source of truth: **`package.json`** (bumped by Changesets).
- Landing site is static HTML/CSS, hosted on Vercel, no build step today. Changelog is
  **prerendered at release time** to static HTML — no runtime JS, no CDN dependency.
- Only `index.html` has a `.nav` header; legal pages (`privacy`, `support`, `terms`) use a
  `.legal__back` link. The version badge therefore lives in `index.html` and the new
  `changelog.html` only.
- CI stays entirely out of versioning (matches "CI must not bump"). Solo maintainer; the
  `pre-push` guard is sufficient.

## Architecture / data flow

```
.changeset/*.md            (maintainer authors: bump type + summary)
        │  make release  (local only)
        ▼
changeset version  ─────►  package.json version  +  CHANGELOG.md (root)
        │                          │
        │                          │  node landing/scripts/build-changelog.mjs
        │                          ▼
        │                  landing/changelog.html   (rendered markdown, page template)
        │                  landing/index.html nav   (version badge stamped)
        │                  landing/changelog.html nav(version badge stamped)
        │
   git commit "release: vX.Y.Z"  ─── git push ──►  pre-push guard (no unconsumed changesets)
        ▼
Docker build: read version from package.json, -ldflags -X internal/version.Version=X.Y.Z
        ▼
Go binary (version.Version) ──►  /settings ephemeral footer: "PlusPlus vX.Y.Z"
```

## Components

### 1. Changesets config

- `package.json` at repo root: `private: true`, `name: "plusplus"`, `version: "0.1.0"` (seed),
  devDeps `@changesets/cli` + `husky` + `marked`, scripts:
  - `changeset` → `changeset`
  - `release` → `changeset version && node landing/scripts/build-changelog.mjs`
  - `prepare` → `husky` (installs hooks)
- `.changeset/config.json`: default config, single package, `commit: false`
  (the `make release` flow commits, not Changesets), `access: "restricted"`, `baseBranch: "main"`.
- `CHANGELOG.md` at repo root, seeded with the `0.1.0` entry.
- `.gitignore` += `node_modules`.

### 2. Go binary version

- New package `internal/version/version.go`:
  ```go
  package version
  // Version is set via -ldflags at build time; "dev" for local builds.
  var Version = "dev"
  ```
- `Dockerfile`: copy `package.json`, extract version, pass through ldflags:
  ```dockerfile
  COPY package.json ./
  RUN VERSION="$(grep '"version"' package.json | head -1 | sed -E 's/.*"version" *: *"([^"]+)".*/\1/')" \
      && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
         go build -ldflags "-X plusplus/internal/version.Version=${VERSION}" -o /bin/plusplus ./cmd/server
  ```
  Local `go run` / `go build` (Makefile `run`/`build`) stay unflagged → `Version == "dev"`.

### 3. `/settings` version footer

- Thread `version.Version` from `main.go` into `NewCommandsProcessor` (new field), then into
  `respondSettings*` → `settingsBlocks`.
- `settingsBlocks` appends a trailing `context` block:
  ```json
  { "type": "context", "elements": [ { "type": "mrkdwn", "text": "PlusPlus vX.Y.Z" } ] }
  ```
- Tests: `settings_blocks` / `commands_processor` tests updated to assert the context block and
  that a provided version string is rendered. `dev` is acceptable in tests.

### 4. Landing changelog prerender

- `landing/scripts/build-changelog.mjs`:
  - Reads root `CHANGELOG.md`, renders with `marked`.
  - Wraps output in a page template that reuses the existing `.nav` header (brand + GitHub +
    version badge) and the existing footer markup/styles from `index.html`. New content section
    uses existing typographic styles where possible; minimal additions to `styles.css` if needed
    for changelog list spacing.
  - Reads version from `package.json`, writes it into the nav badge of both the generated
    `changelog.html` and (via marker replace) `index.html`.
  - Idempotent: re-running produces identical output for the same inputs.
- Version badge markup (canonical, marker-based replace):
  ```html
  <a class="nav__link nav__version" data-version href="changelog.html">vX.Y.Z</a>
  ```
  Script replaces the element's text (`vX.Y.Z`) based on `data-version` attribute.

### 5. Release flow & hooks

- `Makefile` `release` target → `npm run release` (then maintainer reviews `git diff`,
  commits `release: vX.Y.Z`). `release` added to `.PHONY`.
- **Husky** manages hooks via `package.json` `prepare`.
- `.husky/pre-push`: fail-closed when pushing `main` with unconsumed changesets present in
  `.changeset/` (any `*.md` other than `README.md`). Message tells the maintainer to run
  `make release`. Pure guard — no file mutation, no commits.

## Error handling

- `build-changelog.mjs` fails loudly (non-zero exit) if `CHANGELOG.md` or `package.json` is
  missing or the version can't be parsed. No silent fallback.
- `pre-push` hook fails closed (blocks push) when unconsumed changesets exist on a `main` push.
- Go: missing ldflags is not an error — `Version` defaults to `"dev"`.

## Testing

- Go: unit tests for the `/settings` version context block (existing test files).
- `make build` / `go vet ./...` must pass.
- Manual: `npm run release` on a dummy changeset bumps `package.json` + `CHANGELOG.md` and
  regenerates `changelog.html` with the new version in both navs.
- Manual: `git push` with an unconsumed changeset is blocked by the hook.

## Out of scope (YAGNI)

- Git tags / GitHub Releases automation.
- CI workflow for the Go app (none exists today; not introduced here).
- Version badge on legal pages (no `.nav` there).
- Multi-package / monorepo Changesets features.
- Runtime/client-side markdown rendering.

## Files

New:
- `package.json`
- `.changeset/config.json`
- `CHANGELOG.md`
- `.husky/pre-push`
- `landing/scripts/build-changelog.mjs`
- `landing/changelog.html` (generated, committed)
- `internal/version/version.go`

Edited:
- `Dockerfile` (ldflags + copy package.json)
- `Makefile` (`release` target)
- `.gitignore` (`node_modules`)
- `internal/slack/settings_blocks.go`, `internal/slack/commands_processor.go`,
  `cmd/server/main.go` (version footer plumbing)
- `internal/slack/settings_blocks_test.go` / `commands_processor_test.go` (assertions)
- `landing/index.html` (nav version badge + Changelog link)
- `landing/styles.css` (badge + changelog content styles, if needed)
