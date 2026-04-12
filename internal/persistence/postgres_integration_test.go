//go:build integration

package persistence

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestPostgresRepositoriesIntegration(t *testing.T) {
	databaseURL := getenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/plusplus?sslmode=disable")
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping postgres: %v", err)
	}
	if err := RunMigrations(ctx, db); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	teamID := "T-it"
	if _, err := db.ExecContext(ctx, `DELETE FROM karma_totals WHERE team_id = $1`, teamID); err != nil {
		t.Fatalf("cleanup karma_totals: %v", err)
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM channel_settings WHERE team_id = $1`, teamID); err != nil {
		t.Fatalf("cleanup channel_settings: %v", err)
	}

	karmaRepo := NewPostgresKarmaRepository(db)
	settingsRepo := NewPostgresSettingsRepository(db)

	record, err := karmaRepo.ApplyDelta(ctx, teamID, "U1", 3)
	if err != nil {
		t.Fatalf("apply delta: %v", err)
	}
	if record.KarmaTotal != 3 {
		t.Fatalf("expected total 3, got %d", record.KarmaTotal)
	}
	if _, err := karmaRepo.ApplyDelta(ctx, teamID, "U2", 5); err != nil {
		t.Fatalf("apply delta for U2: %v", err)
	}

	leaderboard, err := karmaRepo.GetLeaderboard(ctx, teamID, 10)
	if err != nil {
		t.Fatalf("get leaderboard: %v", err)
	}
	if len(leaderboard) != 2 {
		t.Fatalf("expected 2 leaderboard rows, got %d", len(leaderboard))
	}
	if leaderboard[0].UserID != "U2" {
		t.Fatalf("expected U2 first, got %s", leaderboard[0].UserID)
	}

	mode, err := settingsRepo.GetReplyMode(ctx, teamID, "C1")
	if err != nil {
		t.Fatalf("default reply mode: %v", err)
	}
	if mode != "thread" {
		t.Fatalf("expected thread default, got %s", mode)
	}

	if err := settingsRepo.SetReplyMode(ctx, teamID, "C1", "U-admin", "channel"); err != nil {
		t.Fatalf("set reply mode: %v", err)
	}
	mode, err = settingsRepo.GetReplyMode(ctx, teamID, "C1")
	if err != nil {
		t.Fatalf("read updated reply mode: %v", err)
	}
	if mode != "channel" {
		t.Fatalf("expected channel mode, got %s", mode)
	}
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
