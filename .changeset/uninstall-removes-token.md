---
"plusplus": patch
---

When a workspace removes PlusPlus, its stored Slack token is now deleted. Karma totals and channel settings are kept, so reinstalling brings everything back. Self-hosters using OAuth mode: subscribe your Slack app to the `app_uninstalled` and `tokens_revoked` bot events (see "Upgrading to uninstall cleanup" in the self-hosting guide). No new scope or AWS permission is needed.
