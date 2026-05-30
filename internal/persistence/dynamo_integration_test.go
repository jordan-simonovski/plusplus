//go:build integration

package persistence

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	appconfig "plusplus/internal/config"
)

// Run against DynamoDB Local:
//
//	DYNAMODB_ENDPOINT=http://localhost:8000 AWS_REGION=us-east-1 \
//	  go test -tags=integration ./internal/persistence -v
func TestDynamoRepositoriesIntegration(t *testing.T) {
	cfg, err := appconfig.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.DynamoDBEndpoint == "" {
		t.Skip("DYNAMODB_ENDPOINT not set; skipping DynamoDB integration test")
	}
	// Isolate from any real tables.
	cfg.KarmaTable = "plusplus_karma_it"
	cfg.SettingsTable = "plusplus_channel_settings_it"
	cfg.WorkspacesTable = "plusplus_workspaces_it"

	ctx := context.Background()
	client, err := NewDynamoClient(ctx, cfg)
	if err != nil {
		t.Fatalf("dynamo client: %v", err)
	}
	if err := EnsureTables(ctx, client, cfg); err != nil {
		t.Fatalf("ensure tables: %v", err)
	}

	karmaRepo := NewDynamoKarmaRepository(client, cfg.KarmaTable)
	settingsRepo := NewDynamoSettingsRepository(client, cfg.SettingsTable)

	teamID := "T-it"

	// Start clean so the test is deterministic across re-runs.
	delKarma := func(userID string) {
		_, _ = client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
			TableName: aws.String(cfg.KarmaTable),
			Key: map[string]types.AttributeValue{
				"team_id": &types.AttributeValueMemberS{Value: teamID},
				"user_id": &types.AttributeValueMemberS{Value: userID},
			},
		})
	}
	delKarma("U1")
	delKarma("U2")
	_, _ = client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(cfg.SettingsTable),
		Key: map[string]types.AttributeValue{
			"team_id":    &types.AttributeValueMemberS{Value: teamID},
			"channel_id": &types.AttributeValueMemberS{Value: "C-it"},
		},
	})

	record, err := karmaRepo.ApplyDelta(ctx, teamID, "U1", 3)
	if err != nil {
		t.Fatalf("apply delta: %v", err)
	}
	if record.KarmaTotal != 3 {
		t.Fatalf("expected total 3, got %d", record.KarmaTotal)
	}
	if record.KarmaMax != 3 {
		t.Fatalf("expected max 3, got %d", record.KarmaMax)
	}

	// Drop then re-add to confirm karma_max holds the peak.
	if _, err := karmaRepo.ApplyDelta(ctx, teamID, "U1", -2); err != nil {
		t.Fatalf("apply negative delta: %v", err)
	}
	peaked, err := karmaRepo.ApplyDelta(ctx, teamID, "U1", 0)
	if err != nil {
		t.Fatalf("apply zero delta: %v", err)
	}
	if peaked.KarmaTotal != 1 {
		t.Fatalf("expected total 1, got %d", peaked.KarmaTotal)
	}
	if peaked.KarmaMax != 3 {
		t.Fatalf("expected max to stay 3, got %d", peaked.KarmaMax)
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

	mode, level, err := settingsRepo.GetChannelSettings(ctx, teamID, "C-it")
	if err != nil {
		t.Fatalf("default settings: %v", err)
	}
	if mode != "thread" || level != 5 {
		t.Fatalf("expected thread/5 defaults, got %s/%d", mode, level)
	}

	if err := settingsRepo.SetReplyMode(ctx, teamID, "C-it", "U-admin", "channel"); err != nil {
		t.Fatalf("set reply mode: %v", err)
	}
	mode, level, err = settingsRepo.GetChannelSettings(ctx, teamID, "C-it")
	if err != nil {
		t.Fatalf("read updated settings: %v", err)
	}
	if mode != "channel" {
		t.Fatalf("expected channel mode, got %s", mode)
	}
	if level != 5 {
		t.Fatalf("expected snark default 5 seeded on insert, got %d", level)
	}

	// Updating snark must not clobber the previously set reply mode.
	if err := settingsRepo.SetSnarkLevel(ctx, teamID, "C-it", "U-admin", 8); err != nil {
		t.Fatalf("set snark level: %v", err)
	}
	mode, level, err = settingsRepo.GetChannelSettings(ctx, teamID, "C-it")
	if err != nil {
		t.Fatalf("read settings after snark: %v", err)
	}
	if mode != "channel" || level != 8 {
		t.Fatalf("expected channel/8, got %s/%d", mode, level)
	}
}
