---
"plusplus": patch
---

Karma is no longer given twice when Slack re-sends a message, which it does when a reply takes more than a few seconds (for example, karma for a large user group). Self-hosters: the app now keeps a small record per karma message for 24 hours. To have DynamoDB delete these automatically, turn on TTL for the `expires_at` attribute on the karma table (see "Expiring event claims" in the self-hosting guide); without it everything still works and the records accumulate.
