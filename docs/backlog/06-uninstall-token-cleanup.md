# 06. Delete the token on uninstall

Do after 04.

## Problem

When a workspace removes the app, Slack sends `app_uninstalled` (and `tokens_revoked` when
tokens are revoked). The code handles neither: searching the repository for either name
finds nothing. The encrypted bot token row in the workspaces table stays forever. The
token is dead, but keeping credentials for workspaces that left is poor hygiene for a
Marketplace app, and `landing/privacy.html` tells users their data is retained "while the
app is installed".

## Change

1. Add `DeleteInstallation(ctx, teamID) error` to `DynamoWorkspaceRepository`. Deleting a
   row that does not exist is not an error.
2. In `ProcessEvent`, after signature verification, handle two event types inside
   `event_callback`:
   - `app_uninstalled`: delete the installation for `envelope.TeamID`.
   - `tokens_revoked`: delete it only when the event's `tokens.bot` list is non-empty.
     Add the field to `SlackEvent`.
   Respond 200 on success and 500 on a storage error (deleting twice is harmless, so a
   retry is safe).
3. Declare the dependency as a one-method interface in `internal/slack` and attach it with
   a `With…` method. In single-workspace mode there is no store; the events are then
   acknowledged with 200 and ignored.
4. Add both events to `bot_events` in `slack-app-manifest.yaml` and
   `slack-app-manifest.self-hosted.yaml`. Neither needs a new scope. This is a change to
   Slack app configuration (rule 8 in `AGENTS.md`): the task authorises the manifest edit,
   and the owner must also add the subscriptions to the live app. Say so at the top of the
   PR description.
5. Update the upgrade notes in `docs/self-hosting.md` so self-hosters know to add the two
   event subscriptions.

## Owner decision

This item deletes only the token. Karma totals and settings are kept, so a workspace that
reinstalls gets its history back. Deleting karma on uninstall would be a product and
privacy-policy change; do not do it here.

## Tests

- Unit: `app_uninstalled` calls delete with the envelope's team; `tokens_revoked` with bot
  tokens deletes and without them does not; an unsigned request deletes nothing; with no
  store configured the response is 200.
- Integration: after `DeleteInstallation`, `GetBotToken` returns `domain.ErrNotFound`, and
  deleting again succeeds.

## Done when

- Tests above pass.
- Both manifests list the two events.
- Changeset (patch): PlusPlus now deletes a workspace's stored Slack token when the app is
  uninstalled. Self-hosters using OAuth mode should add the two event subscriptions.
