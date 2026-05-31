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
