---
"plusplus": minor
---

Slack Marketplace review fixes:

- Slash commands renamed to `/plusplus-leaderboard` and `/plusplus-settings` (old names still accepted for self-hosted apps). Both reply with usage and a support link for `help` or unknown input.
- The landing page "Add to Slack" buttons go through `/slack/install`, so installing needs one OAuth pass instead of two.
- Mentioning the bot next to a karma target (`@sam @plusplus ++`, `@plusplus ++`) no longer credits the bot.
