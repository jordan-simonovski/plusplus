//go:build integration

package persistence

import (
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"plusplus/internal/domain"
)

// Reaction karma depends on DynamoDB returning the set as it was before each
// ADD/DELETE, and on reaction rows staying out of the leaderboard.
func TestDynamoReactionStoreAndOverridesIntegration(t *testing.T) {
	ctx, client, cfg := integrationSetup(t)

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
		var err error
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

	if _, ok, err := store.KarmaTargets(ctx, team, "C1", ts); err != nil || ok {
		t.Fatalf("unrecorded message should report no targets: ok=%v err=%v", ok, err)
	}
	for _, want := range [][]string{{"U2", "U3"}, {}} {
		if err := store.RecordKarmaTargets(ctx, team, "C1", ts, want); err != nil {
			t.Fatal(err)
		}
		got, ok, err := store.KarmaTargets(ctx, team, "C1", ts)
		if err != nil || !ok || strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("karma targets = %v ok=%v err=%v, want %v", got, ok, err, want)
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
