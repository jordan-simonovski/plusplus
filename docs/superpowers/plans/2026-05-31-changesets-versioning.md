# Changesets Semantic Versioning Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Changesets-driven semantic versioning to the Go app, surface the version in the Slack `/settings` output and the landing nav, and add a prerendered landing changelog page.

**Architecture:** `package.json` is the single source of truth (bumped by Changesets). Releases are cut locally via `make release`, which bumps the version, regenerates `CHANGELOG.md`, and prerenders `landing/changelog.html` + stamps the nav badge. The Go binary reads the version at Docker build time via `-ldflags`. A Husky `pre-push` hook blocks pushing `main` with unconsumed changesets. CI is not involved in versioning.

**Tech Stack:** Go, Changesets (`@changesets/cli`), Husky v9, `marked` (node), static HTML/CSS landing site.

---

### Task 1: npm + Changesets + Husky scaffolding

**Files:**
- Create: `package.json`, `.changeset/config.json` (via init), `.changeset/README.md` (via init), `CHANGELOG.md`
- Modify: `.gitignore`

- [ ] **Step 1: Create `package.json`**

```json
{
  "name": "plusplus",
  "version": "0.1.0",
  "private": true,
  "type": "module",
  "description": "Versioning and landing tooling for the PlusPlus Slack karma bot (Go app).",
  "scripts": {
    "changeset": "changeset",
    "release": "changeset version && node landing/scripts/build-changelog.mjs",
    "build:changelog": "node landing/scripts/build-changelog.mjs",
    "prepare": "husky"
  }
}
```

- [ ] **Step 2: Install dev dependencies**

Run: `npm install --save-dev @changesets/cli husky marked`
Expected: `node_modules/` created, `devDependencies` added to `package.json`, `package-lock.json` created.

- [ ] **Step 3: Initialize Changesets**

Run: `npx changeset init`
Expected: creates `.changeset/config.json` and `.changeset/README.md`.

- [ ] **Step 4: Set Changesets base branch**

In `.changeset/config.json`, set `"baseBranch": "main"` (leave `"commit": false` — the `make release` flow commits, not Changesets).

- [ ] **Step 5: Initialize Husky**

Run: `npx husky init`
Expected: creates `.husky/pre-commit` (sample) and ensures `"prepare": "husky"` is in `package.json`.
Then delete the sample: `rm .husky/pre-commit` (we use `pre-push`, added in Task 2).

- [ ] **Step 6: Seed `CHANGELOG.md`**

```markdown
# plusplus

## 0.1.0

### Minor Changes

- Initial release. PlusPlus tracks karma in Slack: mention a teammate with `++` to award points and `--` to dock them, with a leaderboard, per-channel reply mode, and adjustable snark.
```

- [ ] **Step 7: Ignore `node_modules`**

Append to `.gitignore`:
```
node_modules
```

- [ ] **Step 8: Verify Changesets runs**

Run: `npx changeset status --output /dev/stdout || true`
Expected: command runs without error (no pending changesets is fine).

- [ ] **Step 9: Commit**

```bash
git add package.json package-lock.json .changeset .husky CHANGELOG.md .gitignore
git commit -m "build: add Changesets, Husky, and marked tooling"
```

---

### Task 2: pre-push guard hook

**Files:**
- Create: `.husky/pre-push`

- [ ] **Step 1: Write the hook**

Create `.husky/pre-push` (Husky v9 — plain script, no sourcing):

```sh
# Block pushing to main when unconsumed changesets exist.
# Run `make release` to consume them before pushing.
protected="main"

unconsumed=$(ls .changeset/*.md 2>/dev/null | grep -v 'README.md$' || true)

blocked=0
while read -r local_ref local_sha remote_ref remote_sha; do
  case "$remote_ref" in
    refs/heads/"$protected")
      if [ -n "$unconsumed" ]; then
        blocked=1
      fi
      ;;
  esac
done

if [ "$blocked" -eq 1 ]; then
  echo "pre-push: unconsumed changesets present; run 'make release' before pushing to $protected:" >&2
  echo "$unconsumed" >&2
  exit 1
fi

exit 0
```

- [ ] **Step 2: Make it executable**

Run: `chmod +x .husky/pre-push`

- [ ] **Step 3: Verify the guard blocks**

```bash
echo "---" > .changeset/zz-test.md   # fake pending changeset
git push --dry-run origin main 2>&1 | grep -q "unconsumed changesets" && echo GUARD_OK
rm .changeset/zz-test.md
```
Expected: prints `GUARD_OK` (push blocked while a changeset is pending). If no remote/upstream is configured, instead run the hook directly:
`printf 'refs/heads/main x refs/heads/main y\n' | sh .husky/pre-push; echo "exit=$?"` → expect `exit=1` with a `.changeset/zz-test.md` present, `exit=0` after removing it.

- [ ] **Step 4: Commit**

```bash
git add .husky/pre-push
git commit -m "build: guard pushes to main against unconsumed changesets"
```

---

### Task 3: Go binary version + `/settings` footer (TDD)

**Files:**
- Create: `internal/version/version.go`, `internal/slack/settings_blocks_test.go`
- Modify: `internal/slack/settings_blocks.go`, `internal/slack/commands_processor.go`, `internal/slack/commands_processor_test.go:21,43`, `cmd/server/main.go:60`

- [ ] **Step 1: Create the version package**

`internal/version/version.go`:
```go
package version

// Version is the build version, injected at build time via
// -ldflags "-X plusplus/internal/version.Version=...".
// It defaults to "dev" for local builds.
var Version = "dev"
```

- [ ] **Step 2: Write the failing test for the settings footer**

`internal/slack/settings_blocks_test.go`:
```go
package slack

import "testing"

func TestSettingsBlocksAppendsVersionContext(t *testing.T) {
	blocks := settingsBlocks(5, "1.2.3")

	last := blocks[len(blocks)-1]
	if last["type"] != "context" {
		t.Fatalf("expected last block type context, got %v", last["type"])
	}

	elements, ok := last["elements"].([]interface{})
	if !ok || len(elements) == 0 {
		t.Fatalf("expected context elements, got %v", last["elements"])
	}

	el := elements[0].(map[string]interface{})
	if el["text"] != "PlusPlus v1.2.3" {
		t.Fatalf("unexpected version text: %v", el["text"])
	}
}
```

- [ ] **Step 3: Run the test to verify it fails**

Run: `go test ./internal/slack/ -run TestSettingsBlocksAppendsVersionContext`
Expected: FAIL — `settingsBlocks` currently takes one argument (compile error: too many arguments).

- [ ] **Step 4: Add the version param + context block to `settingsBlocks`**

In `internal/slack/settings_blocks.go`, change the signature and append a context block:
```go
func settingsBlocks(currentLevel int, appVersion string) []map[string]interface{} {
```
At the end, replace the final `return` with:
```go
	return []map[string]interface{}{
		{
			"type": "section",
			"text": map[string]interface{}{
				"type": "mrkdwn",
				"text": "*Channel settings*\n• Reply mode: `/settings reply_mode thread|channel`\n• Snark level: choose below (1 = dry, 10 = spicy).",
			},
		},
		{
			"type":     "actions",
			"elements": []interface{}{selectEl},
		},
		{
			"type": "context",
			"elements": []interface{}{
				map[string]interface{}{
					"type": "mrkdwn",
					"text": fmt.Sprintf("PlusPlus v%s", appVersion),
				},
			},
		},
	}
```

- [ ] **Step 5: Thread the version through `CommandsProcessor`**

In `internal/slack/commands_processor.go`:
- Add a field to the struct:
```go
type CommandsProcessor struct {
	signingSecret   string
	leaderboard     LeaderboardService
	settingsService SettingsCommandService
	version         string
}
```
- Update the constructor:
```go
func NewCommandsProcessor(signingSecret string, leaderboard LeaderboardService, settingsService SettingsCommandService, version string) *CommandsProcessor {
	return &CommandsProcessor{
		signingSecret:   signingSecret,
		leaderboard:     leaderboard,
		settingsService: settingsService,
		version:         version,
	}
}
```
- In `respondSettingsInteractive`, pass the version:
```go
	writeJSON(w, http.StatusOK, MessageResponse{
		ResponseType: "ephemeral",
		Text:         "Configure this channel's karma replies.",
		Blocks:       settingsBlocks(current, p.version),
	})
```

- [ ] **Step 6: Fix the two existing test call sites**

In `internal/slack/commands_processor_test.go`, update both `NewCommandsProcessor` calls (lines 21 and 43) to pass a version:
```go
processor := NewCommandsProcessor("secret", fakeLeaderboardService{}, fakeSettingsCommandService{}, "test")
```

- [ ] **Step 7: Wire the real version in `main.go`**

In `cmd/server/main.go`:
- Add import `"plusplus/internal/version"`.
- Update the `NewCommandsProcessor` call (line ~60):
```go
		transport.NewCommandsHandler(appslack.NewCommandsProcessor(cfg.SlackSigningSecret, karmaService, settingsService, version.Version)),
```

- [ ] **Step 8: Run tests to verify they pass**

Run: `go test ./internal/slack/ && go vet ./... && go build ./...`
Expected: PASS, no vet errors, build succeeds.

- [ ] **Step 9: Commit**

```bash
git add internal/version/version.go internal/slack/settings_blocks.go internal/slack/settings_blocks_test.go internal/slack/commands_processor.go internal/slack/commands_processor_test.go cmd/server/main.go
git commit -m "feat: surface app version in /settings output"
```

---

### Task 4: Bake version into the Docker build

**Files:**
- Modify: `Dockerfile:5-11`

- [ ] **Step 1: Copy `package.json` and add ldflags**

Replace the build section of `Dockerfile` (the `COPY go.mod ...` through the `RUN ... go build ...` lines) with:
```dockerfile
COPY go.mod go.sum package.json ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal

RUN VERSION="$(sed -n 's/.*"version" *: *"\([^"]*\)".*/\1/p' package.json | head -n1)" \
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
       go build -ldflags "-X plusplus/internal/version.Version=${VERSION}" -o /bin/plusplus ./cmd/server
```

- [ ] **Step 2: Verify the image builds and reports the version**

Run: `docker build -t plusplus:verify . && docker run --rm --entrypoint /bin/plusplus plusplus:verify --version 2>/dev/null || true`
Expected: image builds successfully. (The binary has no `--version` flag; this step only verifies the build. The baked value is confirmed via `/settings` at runtime.)
Fallback if Docker is unavailable: `VERSION=0.1.0 go build -ldflags "-X plusplus/internal/version.Version=$VERSION" -o /tmp/plusplus ./cmd/server && echo BUILD_OK`.

- [ ] **Step 3: Commit**

```bash
git add Dockerfile
git commit -m "build: bake package.json version into the binary via ldflags"
```

---

### Task 5: Landing changelog prerender + nav badge

**Files:**
- Create: `landing/scripts/build-changelog.mjs`, `landing/changelog.html` (generated)
- Modify: `landing/index.html:30-36`, `landing/styles.css`

- [ ] **Step 1: Add the version badge + Changelog link to the index nav**

In `landing/index.html`, replace the `<nav class="nav">...</nav>` block (lines 30-36) with:
```html
    <nav class="nav">
      <a class="brand" href="index.html">
        <img class="brand__logo" src="plusplus-icon.png" alt="PlusPlus logo" width="36" height="36" />
        <span class="brand__name">plusplus</span>
      </a>
      <div class="nav__links">
        <a class="nav__link nav__version" data-version href="changelog.html">v0.1.0</a>
        <a class="nav__link" href="changelog.html">Changelog</a>
        <a class="nav__link" href="https://github.com/jordan-simonovski/plusplus" target="_blank" rel="noopener">GitHub</a>
      </div>
    </nav>
```

- [ ] **Step 2: Add nav + changelog styles**

Append to `landing/styles.css`:
```css
.nav__links {
  display: flex;
  align-items: center;
  gap: 20px;
}

.nav__version {
  border: 1px solid var(--border);
  border-radius: 999px;
  padding: 3px 10px;
  font-size: 0.82rem;
  font-variant-numeric: tabular-nums;
}

.changelog {
  max-width: 760px;
  margin: 0 auto;
}
.changelog h2 {
  margin-top: 2.2rem;
  font-size: 1.4rem;
}
.changelog h3 {
  margin-top: 1.4rem;
  color: var(--muted);
  font-size: 1rem;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}
.changelog ul { padding-left: 1.2rem; }
.changelog li { margin: 0.4rem 0; }
.changelog a { color: var(--text); }
```

- [ ] **Step 3: Write the prerender script**

Create `landing/scripts/build-changelog.mjs`:
```js
import { readFileSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";
import { marked } from "marked";

const here = dirname(fileURLToPath(import.meta.url));
const repoRoot = join(here, "..", "..");
const landingDir = join(repoRoot, "landing");

function fail(msg) {
  console.error(`build-changelog: ${msg}`);
  process.exit(1);
}

let pkg;
try {
  pkg = JSON.parse(readFileSync(join(repoRoot, "package.json"), "utf8"));
} catch (e) {
  fail(`cannot read package.json: ${e.message}`);
}
const version = pkg.version;
if (!version) fail("package.json has no version");

let changelogMd;
try {
  changelogMd = readFileSync(join(repoRoot, "CHANGELOG.md"), "utf8");
} catch (e) {
  fail(`cannot read CHANGELOG.md: ${e.message}`);
}

const body = marked.parse(changelogMd);

const page = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <meta name="slack-app-id" content="A0AS7F57GJH" />
  <title>Changelog — PlusPlus</title>
  <meta name="description" content="Release history and version notes for the PlusPlus Slack karma bot." />
  <meta name="robots" content="index, follow" />
  <link rel="icon" type="image/png" href="plusplus-icon.png" />
  <link rel="stylesheet" href="styles.css" />
</head>
<body>
  <header class="hero">
    <nav class="nav">
      <a class="brand" href="index.html">
        <img class="brand__logo" src="plusplus-icon.png" alt="PlusPlus logo" width="36" height="36" />
        <span class="brand__name">plusplus</span>
      </a>
      <div class="nav__links">
        <a class="nav__link nav__version" data-version href="changelog.html">v${version}</a>
        <a class="nav__link" href="changelog.html">Changelog</a>
        <a class="nav__link" href="https://github.com/jordan-simonovski/plusplus" target="_blank" rel="noopener">GitHub</a>
      </div>
    </nav>
  </header>
  <main>
    <article class="changelog section">
      <h1 class="section__title">Changelog</h1>
${body}
    </article>
  </main>
  <footer class="footer">
    <span>PlusPlus — a Slack karma bot.</span>
    <span class="footer__links">
      <a href="support.html">Support</a>
      <a href="privacy.html">Privacy</a>
      <a href="terms.html">Terms</a>
      <a href="https://github.com/jordan-simonovski/plusplus" target="_blank" rel="noopener">Source on GitHub</a>
    </span>
  </footer>
</body>
</html>
`;

writeFileSync(join(landingDir, "changelog.html"), page);

const indexPath = join(landingDir, "index.html");
let index = readFileSync(indexPath, "utf8");
const badgeRe = /<a class="nav__link nav__version"[^>]*>v[^<]*<\/a>/;
if (!badgeRe.test(index)) {
  fail("index.html missing the nav__version badge marker");
}
index = index.replace(
  badgeRe,
  `<a class="nav__link nav__version" data-version href="changelog.html">v${version}</a>`
);
writeFileSync(indexPath, index);

console.log(`build-changelog: wrote changelog.html and stamped index nav with v${version}`);
```

- [ ] **Step 4: Generate and verify**

Run: `node landing/scripts/build-changelog.mjs`
Expected: prints `build-changelog: wrote changelog.html and stamped index nav with v0.1.0`; `landing/changelog.html` exists and contains the seeded changelog content; `landing/index.html` nav badge reads `v0.1.0`.
Verify idempotency: run it a second time and confirm `git diff --stat landing/` shows no further changes.

- [ ] **Step 5: Commit**

```bash
git add landing/scripts/build-changelog.mjs landing/changelog.html landing/index.html landing/styles.css
git commit -m "feat: prerendered landing changelog page and nav version badge"
```

---

### Task 6: `make release` target + end-to-end verification

**Files:**
- Modify: `Makefile:5`

- [ ] **Step 1: Add the `release` target**

In `Makefile`, add `release` to the `.PHONY` line:
```makefile
.PHONY: fmt lint test test-integration run build up down logs release
```
And add the target at the end:
```makefile
release:
	npm run release
```

- [ ] **Step 2: End-to-end release dry run**

Author a real changeset (a patch bump), then run the release:
```bash
cat > .changeset/test-bump.md <<'EOF'
---
"plusplus": patch
---

Dry-run changeset to verify the release flow.
EOF
make release
```
Expected: `package.json` version bumps `0.1.0` → `0.1.1`, `CHANGELOG.md` gains a `## 0.1.1` entry, `.changeset/test-bump.md` is consumed (deleted), `landing/changelog.html` regenerates with `v0.1.1`, and `landing/index.html` nav badge updates to `v0.1.1`. Inspect with `git diff --stat`.
Then revert the dry-run bump so only the tooling lands:
```bash
git checkout -- package.json CHANGELOG.md landing/changelog.html landing/index.html
```
(`.changeset/test-bump.md` was already consumed by `changeset version`, so nothing to clean there.) OR keep the bump intentionally if a real `0.1.1` release is desired.

- [ ] **Step 3: Full verification**

Run: `go test ./... && go vet ./... && go build ./...`
Expected: all pass.

- [ ] **Step 4: Commit**

```bash
git add Makefile
git commit -m "build: add make release target for local version cuts"
```

---

## Notes for the implementer

- The version flows one way: `.changeset/*.md` → `package.json` (via `changeset version`) → `CHANGELOG.md` + landing artifacts (via the prerender script) → Go binary (via Docker ldflags) → `/settings`. Don't hand-edit `package.json` version.
- The `pre-push` hook is a guard only; it never mutates files. Cutting a release is always an explicit `make release`.
- Local `go run`/`go build` (Makefile `run`/`build`) intentionally leave `version.Version == "dev"`; only the Docker build stamps a real version.
