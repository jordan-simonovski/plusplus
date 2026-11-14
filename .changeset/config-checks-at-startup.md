---
"plusplus": patch
---

The server now refuses to start, with a message saying what to fix, when `SLACK_SIGNING_SECRET` is missing, when neither `SLACK_BOT_TOKEN` nor `SLACK_CLIENT_ID` is set, or when `MAX_KARMA_PER_ACTION` isn't a whole number. Before, it started anyway: without the Slack settings it rejected every request or failed every reply, and an unreadable `MAX_KARMA_PER_ACTION` was silently treated as 5. Self-hosters: a deployment that was misconfigured in one of these ways and still ran will now stop at startup; check the logs for the reason. The unused `SLACK_APP_TOKEN` setting is no longer read.
