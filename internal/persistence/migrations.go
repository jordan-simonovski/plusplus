package persistence

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func RunMigrations(ctx context.Context, db *sql.DB) error {
	log.Printf("migrations: starting")
	if _, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
  version TEXT PRIMARY KEY,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`); err != nil {
		return fmt.Errorf("ensure schema_migrations table: %w", err)
	}

	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations directory: %w", err)
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		files = append(files, filepath.Join("migrations", entry.Name()))
	}
	sort.Strings(files)
	log.Printf("migrations: discovered %d file(s)", len(files))

	appliedCount := 0
	skippedCount := 0

	for _, migrationFile := range files {
		applied, err := migrationApplied(ctx, db, migrationFile)
		if err != nil {
			return err
		}
		if applied {
			skippedCount++
			log.Printf("migrations: already applied %s", migrationFile)
			continue
		}

		migrationSQL, err := migrationsFS.ReadFile(migrationFile)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", migrationFile, err)
		}

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", migrationFile, err)
		}

		if _, err := tx.ExecContext(ctx, string(migrationSQL)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %s: %w", migrationFile, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(version) VALUES ($1)`, migrationFile); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %s: %w", migrationFile, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", migrationFile, err)
		}
		appliedCount++
		log.Printf("migrations: applied %s", migrationFile)
	}

	log.Printf("migrations: complete (applied=%d skipped=%d)", appliedCount, skippedCount)
	return nil
}

func migrationApplied(ctx context.Context, db *sql.DB, version string) (bool, error) {
	var exists bool
	err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)`, version).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check migration %s: %w", version, err)
	}
	return exists, nil
}
