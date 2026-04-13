ALTER TABLE channel_settings
  ADD COLUMN IF NOT EXISTS snark_level INTEGER NOT NULL DEFAULT 5
    CHECK (snark_level >= 1 AND snark_level <= 10);
