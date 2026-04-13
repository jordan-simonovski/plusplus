CREATE TABLE IF NOT EXISTS slack_workspaces (
  team_id TEXT NOT NULL PRIMARY KEY,
  bot_token_ciphertext BYTEA NOT NULL,
  installed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
