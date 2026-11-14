# plusplus

## 1.1.0

### Minor Changes

- 307eb05: Reacting to a karma message now gives the karma to the people it mentions, not the person who posted it. React 🔥 to `@plusplus @sam ++` and Sam gets the 4 karma. The poster's own reactions on that message don't count, and reactions to a user group award give nothing.
- 28a4499: The one-off tool for moving data from the old Postgres (Supabase) database to DynamoDB has been removed, along with its Postgres dependency. Everyone has long since moved to DynamoDB; if you still need to migrate an old Postgres instance, the README names the last commit that has the tool.

### Patch Changes

- 6abeba5: The server now refuses to start, with a message saying what to fix, when `SLACK_SIGNING_SECRET` is missing, when neither `SLACK_BOT_TOKEN` nor `SLACK_CLIENT_ID` is set, or when `MAX_KARMA_PER_ACTION` isn't a whole number. Before, it started anyway: without the Slack settings it rejected every request or failed every reply, and an unreadable `MAX_KARMA_PER_ACTION` was silently treated as 5. Self-hosters: a deployment that was misconfigured in one of these ways and still ran will now stop at startup; check the logs for the reason. The unused `SLACK_APP_TOKEN` setting is no longer read.
- 01f05bb: The container image is now built with Go 1.26.11, which includes the latest Go security fixes. Self-hosters pick these up by pulling the new image.
- cafae6d: Karma for a user group (`@plusplus @team ++`) now works for groups of up to 50 people. For a bigger group PlusPlus replies that it's too large instead of giving everyone karma one by one, which was slow enough for Slack to give up on and produced replies too long to post.
- b358659: `/plusplus-settings help` now starts by reminding you to invite the bot to a channel with `/invite @plusplus`. Self-hosted: the setup guide now explains that plain `@someone ++` only works in channels the bot has been invited to.
- c2c0968: Karma is no longer given twice when Slack re-sends a message, which it does when a reply takes more than a few seconds (for example, karma for a large user group). Self-hosters: the app now keeps a small record per karma message for 24 hours. To have DynamoDB delete these automatically, turn on TTL for the `expires_at` attribute on the karma table (see "Expiring event claims" in the self-hosting guide); without it everything still works and the records accumulate.
- 44cb06e: Links to pluspluskarma.dev now show an up-to-date preview card when shared in Slack or on social media, including links to the support, changelog, privacy and terms pages.
- 3301c29: When something goes wrong (a failed karma update, a Slack reply that didn't send, a settings change that didn't save), PlusPlus now logs why, with the workspace it happened in. Self-hosters: logs are now JSON lines (one object per line, with `level`, `msg`, `team`, `kind` and `err` fields) instead of plain text, so update any log filters that matched the old wording. Message text is never logged.
- 2ce63fc: Stored Slack tokens are now encrypted so they only work for the workspace they belong to. Existing tokens keep working and are upgraded automatically the next time each workspace uses PlusPlus. Self-hosters: once you run this version, don't downgrade to an earlier one, because earlier versions can't read upgraded tokens; those workspaces would have to reinstall.
- 8474c19: When a workspace removes PlusPlus, its stored Slack token is now deleted. Karma totals and channel settings are kept, so reinstalling brings everything back. Self-hosters using OAuth mode: subscribe your Slack app to the `app_uninstalled` and `tokens_revoked` bot events (see "Upgrading to uninstall cleanup" in the self-hosting guide). No new scope or AWS permission is needed.

## 1.0.0

### Major Changes

- PlusPlus 1.0 is here, and it's now on the [Slack Marketplace](https://slack.com/marketplace/A0AS7F57GJH-plusplus-karma). From this release on, anything that changes how you use PlusPlus or would break a self-hosted install will come in a new major version.

### Patch Changes

- Self-hosted: version tags on the container image, like `1.0.0`, now always point at that release and never change after it ships.

## 0.3.0

### Minor Changes

- React to a teammate's message to give them karma: 👏 +1, 🙌 +2, ⭐ +3, 🔥 +4, 🏆 +5. 🙄 👎 🤡 💩 🗑️ take 1 to 5 away. Your strongest reaction on a message is the one that counts, and removing it takes the karma back.
- Swap in your own emojis, custom ones included, for one channel with `/plusplus-settings emoji`. Workspace admins can change them for everyone.
- Already using PlusPlus? Have a workspace admin add it to Slack again to turn on reactions.
- Typed karma now works by mentioning PlusPlus: `@plusplus @sam ++`. Self-hosted installs can keep plain `@sam ++`.

### Patch Changes

- Refreshed the website. The demo now shows exactly what the bot replies, including karma feuds and capped awards, and the page is easier to read and use on phones and with a keyboard.
- `/plusplus-settings` now shows which emojis give and take karma in the channel, and how to change them.

## 0.2.0

### Minor Changes

- Slash commands are now `/plusplus-leaderboard` and `/plusplus-settings`. Add `help` to either one to see how to use it. Self-hosted installs can keep using `/leaderboard` and `/settings`.
- Adding PlusPlus to Slack now takes a single authorization step.
- Mentioning PlusPlus next to a teammate, like `@sam @plusplus ++`, now gives the karma to your teammate instead of the bot.

## 0.1.0

### Minor Changes

- Initial release. PlusPlus tracks karma in Slack: mention a teammate with `++` to award points and `--` to dock them, with a leaderboard, per-channel reply mode, and adjustable snark.
