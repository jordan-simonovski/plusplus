---
"plusplus": patch
---

When something goes wrong (a failed karma update, a Slack reply that didn't send, a settings change that didn't save), PlusPlus now logs why, with the workspace it happened in. Self-hosters: logs are now JSON lines (one object per line, with `level`, `msg`, `team`, `kind` and `err` fields) instead of plain text, so update any log filters that matched the old wording. Message text is never logged.
