# Security policy

## Reporting a vulnerability

Please report security problems privately, by email to **contact@pluspluskarma.dev**.
Where GitHub shows a **Report a vulnerability** button on this repository's Security
tab, you can use that instead. Please don't open a public issue.

Include what you found, how to reproduce it, and what an attacker could do with it.
You'll get a reply acknowledging the report.

## Supported versions

Fixes go into the latest release (the newest container image) and the hosted app.
Older versions are not patched; self-hosters should update to the latest image.

## Scope

Of most interest:

- Slack request signature verification
- Isolation between workspaces (one workspace reading or changing another's data)
- Storage and handling of Slack bot tokens
- The OAuth install flow

Out of scope: rate limiting and denial of service against the hosted app, and anything
that needs a compromised Slack workspace admin account.
