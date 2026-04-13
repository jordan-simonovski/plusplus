package persistence

import (
	"context"
	"database/sql"
	"fmt"

	"plusplus/internal/crypto"
)

// PostgresWorkspaceRepository stores per-workspace Slack bot tokens encrypted at rest.
type PostgresWorkspaceRepository struct {
	db  *sql.DB
	enc *crypto.AESEncryptor
}

func NewPostgresWorkspaceRepository(db *sql.DB, enc *crypto.AESEncryptor) *PostgresWorkspaceRepository {
	return &PostgresWorkspaceRepository{db: db, enc: enc}
}

// UpsertInstallation encrypts and stores the bot token for a workspace.
func (r *PostgresWorkspaceRepository) UpsertInstallation(ctx context.Context, teamID, botToken string) error {
	if teamID == "" {
		return fmt.Errorf("team id empty")
	}
	ct, err := r.enc.Encrypt([]byte(botToken))
	if err != nil {
		return fmt.Errorf("encrypt token: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `
INSERT INTO slack_workspaces (team_id, bot_token_ciphertext, installed_at)
VALUES ($1, $2, NOW())
ON CONFLICT (team_id) DO UPDATE SET
  bot_token_ciphertext = EXCLUDED.bot_token_ciphertext,
  installed_at = NOW()
`, teamID, ct)
	if err != nil {
		return fmt.Errorf("upsert slack workspace: %w", err)
	}
	return nil
}

// GetBotToken decrypts the stored bot token for a workspace.
func (r *PostgresWorkspaceRepository) GetBotToken(ctx context.Context, teamID string) (string, error) {
	var blob []byte
	err := r.db.QueryRowContext(ctx,
		`SELECT bot_token_ciphertext FROM slack_workspaces WHERE team_id = $1`,
		teamID,
	).Scan(&blob)
	if err != nil {
		return "", err
	}
	pt, err := r.enc.Decrypt(blob)
	if err != nil {
		return "", fmt.Errorf("decrypt token: %w", err)
	}
	return string(pt), nil
}
