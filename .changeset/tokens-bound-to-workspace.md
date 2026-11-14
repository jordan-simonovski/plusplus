---
"plusplus": patch
---

Stored Slack tokens are now encrypted so they only work for the workspace they belong to. Existing tokens keep working and are upgraded automatically the next time each workspace uses PlusPlus. Self-hosters: once you run this version, don't downgrade to an earlier one, because earlier versions can't read upgraded tokens; those workspaces would have to reinstall.
