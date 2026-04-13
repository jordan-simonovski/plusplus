package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const (
	defaultReplyMode  = "thread"
	defaultSnarkLevel = 5
	minSnarkLevel     = 1
	maxSnarkLevel     = 10
)

type PostgresSettingsRepository struct {
	db *sql.DB
}

func NewPostgresSettingsRepository(db *sql.DB) *PostgresSettingsRepository {
	return &PostgresSettingsRepository{db: db}
}

func (r *PostgresSettingsRepository) GetChannelSettings(
	ctx context.Context,
	teamID string,
	channelID string,
) (replyMode string, snarkLevel int, err error) {
	const query = `
SELECT reply_mode, snark_level
FROM channel_settings
WHERE team_id = $1 AND channel_id = $2;
`

	var mode string
	var level int
	err = r.db.QueryRowContext(ctx, query, teamID, channelID).Scan(&mode, &level)
	if err != nil {
		if err == sql.ErrNoRows {
			return defaultReplyMode, defaultSnarkLevel, nil
		}
		return "", 0, fmt.Errorf("get channel settings: %w", err)
	}

	if mode == "" {
		mode = defaultReplyMode
	}
	if level < minSnarkLevel || level > maxSnarkLevel {
		level = defaultSnarkLevel
	}
	return mode, level, nil
}

func (r *PostgresSettingsRepository) GetReplyMode(
	ctx context.Context,
	teamID string,
	channelID string,
) (string, error) {
	mode, _, err := r.GetChannelSettings(ctx, teamID, channelID)
	return mode, err
}

func (r *PostgresSettingsRepository) SetReplyMode(
	ctx context.Context,
	teamID string,
	channelID string,
	actorUserID string,
	replyMode string,
) error {
	const query = `
INSERT INTO channel_settings (team_id, channel_id, reply_mode, snark_level, updated_by, updated_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (team_id, channel_id)
DO UPDATE SET
  reply_mode = EXCLUDED.reply_mode,
  updated_by = EXCLUDED.updated_by,
  updated_at = EXCLUDED.updated_at;
`
	_, err := r.db.ExecContext(ctx, query, teamID, channelID, replyMode, defaultSnarkLevel, actorUserID, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("upsert channel settings: %w", err)
	}
	return nil
}

func (r *PostgresSettingsRepository) SetSnarkLevel(
	ctx context.Context,
	teamID string,
	channelID string,
	actorUserID string,
	snarkLevel int,
) error {
	const query = `
INSERT INTO channel_settings (team_id, channel_id, reply_mode, snark_level, updated_by, updated_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (team_id, channel_id)
DO UPDATE SET
  snark_level = EXCLUDED.snark_level,
  updated_by = EXCLUDED.updated_by,
  updated_at = EXCLUDED.updated_at;
`
	_, err := r.db.ExecContext(ctx, query, teamID, channelID, defaultReplyMode, snarkLevel, actorUserID, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("upsert snark level: %w", err)
	}
	return nil
}
