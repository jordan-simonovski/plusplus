package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const defaultReplyMode = "thread"

type PostgresSettingsRepository struct {
	db *sql.DB
}

func NewPostgresSettingsRepository(db *sql.DB) *PostgresSettingsRepository {
	return &PostgresSettingsRepository{db: db}
}

func (r *PostgresSettingsRepository) GetReplyMode(
	ctx context.Context,
	teamID string,
	channelID string,
) (string, error) {
	const query = `
SELECT reply_mode
FROM channel_settings
WHERE team_id = $1 AND channel_id = $2;
`

	var mode string
	err := r.db.QueryRowContext(ctx, query, teamID, channelID).Scan(&mode)
	if err != nil {
		if err == sql.ErrNoRows {
			return defaultReplyMode, nil
		}
		return "", fmt.Errorf("get channel settings: %w", err)
	}

	if mode == "" {
		return defaultReplyMode, nil
	}
	return mode, nil
}

func (r *PostgresSettingsRepository) SetReplyMode(
	ctx context.Context,
	teamID string,
	channelID string,
	actorUserID string,
	replyMode string,
) error {
	const query = `
INSERT INTO channel_settings (team_id, channel_id, reply_mode, updated_by, updated_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (team_id, channel_id)
DO UPDATE SET
  reply_mode = EXCLUDED.reply_mode,
  updated_by = EXCLUDED.updated_by,
  updated_at = EXCLUDED.updated_at;
`
	_, err := r.db.ExecContext(ctx, query, teamID, channelID, replyMode, actorUserID, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("upsert channel settings: %w", err)
	}
	return nil
}
