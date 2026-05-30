// Command migrate is a one-time tool to copy data from the legacy Postgres
// (Supabase) database into DynamoDB. Run it once during the cutover, then it can
// be deleted along with the pgx dependency.
//
// Two input modes:
//
//	# 1) From a pg_dump file (no Postgres needed):
//	AWS_REGION=us-east-1 AWS_ACCESS_KEY_ID=... AWS_SECRET_ACCESS_KEY=... \
//	  go run ./cmd/migrate data.sql
//
//	# 2) From a live Postgres connection:
//	DATABASE_URL=postgres://... \
//	AWS_REGION=us-east-1 AWS_ACCESS_KEY_ID=... AWS_SECRET_ACCESS_KEY=... \
//	  go run ./cmd/migrate
//
// It is idempotent: rows are written with PutItem, so re-running overwrites.
package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	_ "github.com/jackc/pgx/v5/stdlib"

	"plusplus/internal/config"
	"plusplus/internal/persistence"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx := context.Background()

	client, err := persistence.NewDynamoClient(ctx, cfg)
	if err != nil {
		log.Fatalf("init dynamodb client: %v", err)
	}
	if err := persistence.EnsureTables(ctx, client, cfg); err != nil {
		log.Fatalf("ensure dynamodb tables: %v", err)
	}

	var karma, settings, workspaces int

	if dumpPath := firstArg(); dumpPath != "" {
		log.Printf("migrating from dump file %s", dumpPath)
		karma, settings, workspaces = migrateFromDump(ctx, dumpPath, client, cfg)
	} else {
		databaseURL := os.Getenv("DATABASE_URL")
		if databaseURL == "" {
			log.Fatal("provide a pg_dump file path as an argument, or set DATABASE_URL for a live connection")
		}
		log.Printf("migrating from live postgres connection")
		karma, settings, workspaces = migrateFromLive(ctx, databaseURL, client, cfg)
	}

	log.Printf("migration complete: karma=%d settings=%d workspaces=%d", karma, settings, workspaces)
}

func firstArg() string {
	if len(os.Args) > 1 {
		return os.Args[1]
	}
	return ""
}

func migrateFromLive(ctx context.Context, databaseURL string, client *dynamodb.Client, cfg config.Config) (karma, settings, workspaces int) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		log.Fatalf("open postgres: %v", err)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("ping postgres: %v", err)
	}

	karma = migrateKarma(ctx, db, client, cfg.KarmaTable)
	settings = migrateSettings(ctx, db, client, cfg.SettingsTable)
	workspaces = migrateWorkspaces(ctx, db, client, cfg.WorkspacesTable)
	return karma, settings, workspaces
}

func migrateKarma(ctx context.Context, db *sql.DB, client *dynamodb.Client, table string) int {
	rows, err := db.QueryContext(ctx, `SELECT team_id, user_id, karma_total, karma_max, last_activity_at FROM karma_totals`)
	if err != nil {
		log.Fatalf("query karma_totals: %v", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var teamID, userID string
		var total, max int
		var last time.Time
		if err := rows.Scan(&teamID, &userID, &total, &max, &last); err != nil {
			log.Fatalf("scan karma row: %v", err)
		}
		putItem(ctx, client, table, map[string]types.AttributeValue{
			"team_id":          &types.AttributeValueMemberS{Value: teamID},
			"user_id":          &types.AttributeValueMemberS{Value: userID},
			"karma_total":      numAttr(total),
			"karma_max":        numAttr(max),
			"last_activity_at": &types.AttributeValueMemberS{Value: last.UTC().Format(time.RFC3339)},
		})
		count++
	}
	if err := rows.Err(); err != nil {
		log.Fatalf("iterate karma rows: %v", err)
	}
	return count
}

func migrateSettings(ctx context.Context, db *sql.DB, client *dynamodb.Client, table string) int {
	rows, err := db.QueryContext(ctx, `SELECT team_id, channel_id, reply_mode, snark_level, updated_by, updated_at FROM channel_settings`)
	if err != nil {
		log.Fatalf("query channel_settings: %v", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var teamID, channelID, replyMode, updatedBy string
		var snark int
		var updatedAt time.Time
		if err := rows.Scan(&teamID, &channelID, &replyMode, &snark, &updatedBy, &updatedAt); err != nil {
			log.Fatalf("scan settings row: %v", err)
		}
		putItem(ctx, client, table, map[string]types.AttributeValue{
			"team_id":     &types.AttributeValueMemberS{Value: teamID},
			"channel_id":  &types.AttributeValueMemberS{Value: channelID},
			"reply_mode":  &types.AttributeValueMemberS{Value: replyMode},
			"snark_level": numAttr(snark),
			"updated_by":  &types.AttributeValueMemberS{Value: updatedBy},
			"updated_at":  &types.AttributeValueMemberS{Value: updatedAt.UTC().Format(time.RFC3339)},
		})
		count++
	}
	if err := rows.Err(); err != nil {
		log.Fatalf("iterate settings rows: %v", err)
	}
	return count
}

func migrateWorkspaces(ctx context.Context, db *sql.DB, client *dynamodb.Client, table string) int {
	rows, err := db.QueryContext(ctx, `SELECT team_id, bot_token_ciphertext, installed_at FROM slack_workspaces`)
	if err != nil {
		// slack_workspaces may not exist if OAuth was never configured.
		log.Printf("skip workspaces (query failed: %v)", err)
		return 0
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var teamID string
		var ciphertext []byte
		var installedAt time.Time
		if err := rows.Scan(&teamID, &ciphertext, &installedAt); err != nil {
			log.Fatalf("scan workspace row: %v", err)
		}
		putItem(ctx, client, table, map[string]types.AttributeValue{
			"team_id":              &types.AttributeValueMemberS{Value: teamID},
			"bot_token_ciphertext": &types.AttributeValueMemberB{Value: ciphertext},
			"installed_at":         &types.AttributeValueMemberS{Value: installedAt.UTC().Format(time.RFC3339)},
		})
		count++
	}
	if err := rows.Err(); err != nil {
		log.Fatalf("iterate workspace rows: %v", err)
	}
	return count
}

func putItem(ctx context.Context, client *dynamodb.Client, table string, item map[string]types.AttributeValue) {
	if _, err := client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(table),
		Item:      item,
	}); err != nil {
		log.Fatalf("put item into %s: %v", table, err)
	}
}

func numAttr(n int) types.AttributeValue {
	return &types.AttributeValueMemberN{Value: strconv.Itoa(n)}
}
