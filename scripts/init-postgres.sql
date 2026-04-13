CREATE TABLE IF NOT EXISTS karma_totals (
  team_id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  karma_total INTEGER NOT NULL DEFAULT 0,
  karma_max INTEGER NOT NULL DEFAULT 0,
  last_activity_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (team_id, user_id)
);

CREATE INDEX IF NOT EXISTS karma_totals_leaderboard_idx
  ON karma_totals (team_id, karma_total DESC, user_id ASC);

CREATE TABLE IF NOT EXISTS channel_settings (
  team_id TEXT NOT NULL,
  channel_id TEXT NOT NULL,
  reply_mode TEXT NOT NULL CHECK (reply_mode IN ('thread', 'channel')),
  snark_level INTEGER NOT NULL DEFAULT 5 CHECK (snark_level >= 1 AND snark_level <= 10),
  updated_by TEXT NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (team_id, channel_id)
);
