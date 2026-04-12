package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"plusplus/internal/domain"
	"time"
)

type PostgresKarmaRepository struct {
	db *sql.DB
}

func NewPostgresKarmaRepository(db *sql.DB) *PostgresKarmaRepository {
	return &PostgresKarmaRepository{db: db}
}

func (r *PostgresKarmaRepository) ApplyDelta(
	ctx context.Context,
	teamID string,
	userID string,
	delta int,
) (domain.KarmaRecord, error) {
	now := time.Now().UTC()

	const query = `
INSERT INTO karma_totals (team_id, user_id, karma_total, karma_max, last_activity_at)
VALUES ($1, $2, $3, GREATEST($3, 0), $4)
ON CONFLICT (team_id, user_id)
DO UPDATE SET
  karma_total = karma_totals.karma_total + EXCLUDED.karma_total,
  last_activity_at = EXCLUDED.last_activity_at,
  karma_max = GREATEST(karma_totals.karma_max, karma_totals.karma_total + EXCLUDED.karma_total)
RETURNING team_id, user_id, karma_total, karma_max, last_activity_at;
`

	var record domain.KarmaRecord
	var lastActivity time.Time
	err := r.db.QueryRowContext(ctx, query, teamID, userID, delta, now).Scan(
		&record.TeamID,
		&record.UserID,
		&record.KarmaTotal,
		&record.KarmaMax,
		&lastActivity,
	)
	if err != nil {
		return domain.KarmaRecord{}, fmt.Errorf("upsert karma total: %w", err)
	}
	record.LastActivity = lastActivity.Format(time.RFC3339)

	return record, nil
}

func (r *PostgresKarmaRepository) GetLeaderboard(
	ctx context.Context,
	teamID string,
	limit int,
) ([]domain.KarmaRecord, error) {
	const query = `
SELECT team_id, user_id, karma_total, karma_max, last_activity_at
FROM karma_totals
WHERE team_id = $1
ORDER BY karma_total DESC, user_id ASC
LIMIT $2;
`

	rows, err := r.db.QueryContext(ctx, query, teamID, limit)
	if err != nil {
		return nil, fmt.Errorf("query leaderboard: %w", err)
	}
	defer rows.Close()

	records := make([]domain.KarmaRecord, 0, limit)
	for rows.Next() {
		var record domain.KarmaRecord
		var lastActivity time.Time
		if err := rows.Scan(
			&record.TeamID,
			&record.UserID,
			&record.KarmaTotal,
			&record.KarmaMax,
			&lastActivity,
		); err != nil {
			return nil, fmt.Errorf("scan leaderboard row: %w", err)
		}
		record.LastActivity = lastActivity.Format(time.RFC3339)
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate leaderboard rows: %w", err)
	}

	return records, nil
}
