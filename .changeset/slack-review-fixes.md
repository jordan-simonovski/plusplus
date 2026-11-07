---
"plusplus": minor
---

Slack Marketplace review fixes:

- Slash commands renamed to `/plusplus-leaderboard` and `/plusplus-settings` (old names still accepted for self-hosted apps). Both reply with usage and a support link for `help` or unknown input.
- Installing needs one OAuth pass from every entry point: the landing buttons go through `/slack/install`, and Slack-initiated installs (Marketplace, shareable URL) that arrive without `state` are exchanged directly instead of restarting OAuth.
- Mentioning the bot next to a karma target (`@sam @plusplus ++`, `@plusplus ++`) no longer credits the bot.
