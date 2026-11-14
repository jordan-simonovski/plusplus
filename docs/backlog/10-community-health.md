# 10. Contributor and issue scaffolding

Documentation and GitHub configuration only. No Go changes.

## Problem

The repository is public and MIT licensed, and `.github/` holds only two workflows. There
is no contributing guide, security policy, code of conduct, issue forms, PR template or
code owners file. `README.md` opens with environment variables and a Railway deployment
runbook, so a visitor has to read operator notes to find out what the bot does or how to
help.

## Change

Add these files. Keep each short; link rather than repeat.

1. `CONTRIBUTING.md`
   - Prerequisites: Go (version from `go.mod`), Docker, Node (only for changesets).
   - Run it locally: `make up`, `make run`, `make test-integration`.
   - The checks a PR must pass (`make check`, and how to install golangci-lint; see
     `AGENTS.md`).
   - When a changeset is needed and how to write one (`npx changeset`; written for users).
   - Where things live: link to the table in `AGENTS.md` instead of copying it.
   - Using a coding agent is welcome; point it at `AGENTS.md`.
2. `SECURITY.md`
   - Report privately by email to `contact@pluspluskarma.dev`, or through GitHub's "Report
     a vulnerability" button on the Security tab where it is shown (the owner has to
     enable it; see "Owner follow-up"). Do not open a public issue.
   - Supported versions: the latest release and the hosted app.
   - In scope: signature verification, workspace isolation, token storage, the OAuth flow.
3. `CODE_OF_CONDUCT.md`: Contributor Covenant 2.1, contact `contact@pluspluskarma.dev`.
   Fetch the text from
   `https://www.contributor-covenant.org/version/2/1/code_of_conduct/code_of_conduct.md`;
   do not write it from memory. If you cannot fetch it, leave this file out and say so.
4. `.github/ISSUE_TEMPLATE/bug.yml`: an issue form asking for hosted or self-hosted, the
   version (shown at the bottom of the `/plusplus-settings` panel), what was typed or
   reacted, what happened, what was expected, and for self-hosters the relevant log lines
   with a reminder to remove tokens.
5. `.github/ISSUE_TEMPLATE/feature.yml`: the problem, the proposed behaviour, and whether
   it would need a new Slack permission.
6. `.github/ISSUE_TEMPLATE/config.yml`: `blank_issues_enabled: false`, and contact links
   to the support page (`https://pluspluskarma.dev/support.html`) for usage questions and
   to the security policy for vulnerabilities.
7. `.github/pull_request_template.md`: what and why, how it was tested, and a checklist:
   checks pass, changeset added or not needed, `docs/self-hosting.md` updated if
   self-hosters are affected, `/ce-code-review` ran.
8. `.github/CODEOWNERS`: `* @jordan-simonovski`.
9. `README.md`: reorder so it reads top to bottom as: what PlusPlus is, how to use it
   (karma by mention, by reaction, the two slash commands), add it to Slack (Marketplace
   link, already on the landing page), self-host (link to `docs/self-hosting.md`),
   contribute (link to `CONTRIBUTING.md`), licence. Move the sections from "DynamoDB
   Deployment Notes" through "Infrastructure (Terraform)" into a new `docs/operations.md`
   unchanged, and link to it. Do not rewrite the moved content.

Take facts (commands, URLs, version location) from the repository. Do not invent
features, badges or a roadmap.

## Owner follow-up

These are repository settings that files cannot change. List them at the top of the PR
description for the owner:

- The repository's website field points to an unrelated Vercel app
  (`public-open-space-planning-mc1n.vercel.app`). It should be `https://pluspluskarma.dev`.
- The description is "give users points". Suggest one sentence that says what it is.
- No topics are set. Suggest `slack`, `slack-bot`, `karma`, `golang`, `dynamodb`.
- Enable private vulnerability reporting, which `SECURITY.md` relies on.
- Require the `go` workflow's check on `main` through branch protection.

## Done when

- Each file is at the path GitHub's community profile looks for: `CONTRIBUTING.md`,
  `SECURITY.md`, `CODE_OF_CONDUCT.md`, `.github/ISSUE_TEMPLATE/`,
  `.github/pull_request_template.md`, `.github/CODEOWNERS`.
- The issue forms parse as YAML and use only keys from GitHub's issue-form schema.
- Every command and link in the new files has been run or opened.
- No changeset.
