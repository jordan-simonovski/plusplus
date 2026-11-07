# plusplus

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
