//go:build integration

package persistence

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	appconfig "plusplus/internal/config"
	"plusplus/internal/domain"
)

// Reaction karma depends on DynamoDB returning the set as it was before each
// ADD/DELETE, and on reaction rows staying out of the leaderboard.
func TestDynamoReactionStoreAndOverridesIntegration(t *testing.T) {
	cfg, err := appconfig.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.DynamoDBEndpoint == "" {
		t.Skip("DYNAMODB_ENDPOINT not set")
	}
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

	store := NewDynamoReactionStore(client, cfg.KarmaTable)
	// Unique per run so a failed earlier run can't leave state behind.
	team, ts := "T-rx-it", strconv.FormatInt(time.Now().UnixNano(), 10)
	steps := []struct {
		add    bool
		emoji  string
		tier   int
		before string
	}{
		{true, "clap", 1, ""},
		{true, "fire", 4, "clap=1"},
		{true, "fire", 4, "clap=1,fire=4"}, // duplicate add is idempotent
		{false, "clap", 0, "clap=1,fire=4"},
		{false, "fire", 0, "fire=4"},
		{false, "fire", 0, ""}, // removing something never stored is a no-op
		{false, "tada", 0, ""},
	}
	for i, s := range steps {
		var got map[string]int
		if s.add {
			got, err = store.AddReaction(ctx, team, "C1", ts, "U1", s.emoji, s.tier)
		} else {
			got, err = store.RemoveReaction(ctx, team, "C1", ts, "U1", s.emoji)
		}
		if err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
		var pairs []string
		for e, tier := range got {
			pairs = append(pairs, e+"="+strconv.Itoa(tier))
		}
		sort.Strings(pairs)
		if strings.Join(pairs, ",") != s.before {
			t.Fatalf("step %d (%v %s): before = %v, want %q", i, s.add, s.emoji, got, s.before)
		}
	}

	board, err := NewDynamoKarmaRepository(client, cfg.KarmaTable).GetLeaderboard(ctx, team, 10)
	if err != nil {
		t.Fatalf("leaderboard: %v", err)
	}
	if len(board) != 0 {
		t.Fatalf("reaction rows must not appear on the leaderboard, got %v", board)
	}

	settings := NewDynamoSettingsRepository(client, cfg.SettingsTable)
	if err := settings.SetReactionOverrides(ctx, team, "C1", true, "U1", domain.TierOverrides{3: {"partyparrot"}}); err != nil {
		t.Fatal(err)
	}
	if err := settings.SetReactionOverrides(ctx, team, "C1", false, "U1", domain.TierOverrides{-2: {"meh"}}); err != nil {
		t.Fatal(err)
	}
	ch, ws, err := settings.GetReactionOverrides(ctx, team, "C1")
	if err != nil {
		t.Fatal(err)
	}
	if ws[3][0] != "partyparrot" || ch[-2][0] != "meh" {
		t.Fatalf("overrides round-trip: channel=%v workspace=%v", ch, ws)
	}
	if mode, level, err := settings.GetChannelSettings(ctx, team, "C1"); err != nil || mode != "thread" || level != 5 {
		t.Fatalf("emoji overrides must not disturb channel defaults: %s %d %v", mode, level, err)
	}
}
