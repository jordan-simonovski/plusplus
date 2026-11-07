---
"plusplus": minor
---

Slack Marketplace review fixes:

- Slash commands renamed to `/plusplus-leaderboard` and `/plusplus-settings` (old names still accepted for self-hosted apps). Both reply with usage and a support link for `help` or unknown input.
- Karma is now mention-only (`@plusplus @sam ++`). The `channels:history` / `groups:history` scopes and `message.*` events are dropped.
- The landing page "Add to Slack" buttons go through `/slack/install`, so installing needs one OAuth pass instead of two.
