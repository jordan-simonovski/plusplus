package slack

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"plusplus/internal/domain"
)

type defaultTiers struct{}

func (defaultTiers) ReactionTiers(context.Context, string, string) (map[string]int, error) {
	return domain.ResolveReactionTiers(), nil
}

// memReactionState mimics the DynamoDB string-set ADD/DELETE returning the old set.
type memReactionState map[string][]string

func (m memReactionState) AddReaction(_ context.Context, _, ch, ts, who, emoji string) ([]string, error) {
	key := ch + ts + who
	before := append([]string(nil), m[key]...)
	if !contains(m[key], emoji) {
		m[key] = append(m[key], emoji)
	}
	return before, nil
}

func (m memReactionState) RemoveReaction(_ context.Context, _, ch, ts, who, emoji string) ([]string, error) {
	key := ch + ts + who
	before := append([]string(nil), m[key]...)
	var kept []string
	for _, e := range m[key] {
		if e != emoji {
			kept = append(kept, e)
		}
	}
	m[key] = kept
	return before, nil
}

type deltaRecorder struct{ runs *[]string }

func (d deltaRecorder) HandleAction(_ context.Context, a domain.KarmaAction) (domain.KarmaResult, error) {
	*d.runs = append(*d.runs, a.TargetUserID+" "+a.SymbolRun)
	return domain.KarmaResult{Message: "applied"}, nil
}

func reactionEvent(kind, reactor, author, emoji string) []byte {
	return []byte(`{"type":"event_callback","team_id":"T1",
		"authorizations":[{"user_id":"UBOT","is_bot":true}],
		"event":{"type":"` + kind + `","user":"` + reactor + `","reaction":"` + emoji + `","item_user":"` + author + `",
		"item":{"type":"message","channel":"C1","ts":"100.1"},"event_ts":"200.2"}}`)
}

func TestReactionKarmaHighestTierAndUndo(t *testing.T) {
	var runs []string
	web := &fakeWebClient{}
	p := NewEventsProcessor("secret", deltaRecorder{&runs}, nil, nil, web).WithReactions(defaultTiers{}, memReactionState{})

	send := func(kind, reactor, author, emoji string) {
		body := reactionEvent(kind, reactor, author, emoji)
		req := httptest.NewRequest(http.MethodPost, "/slack/events", bytes.NewReader(body))
		addSlackSignatureHeaders(req, "secret", body)
		rec := httptest.NewRecorder()
		p.ProcessEvent(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s %s: status %d", kind, emoji, rec.Code)
		}
	}

	send("reaction_added", "U1", "U2", "clap")              // +1
	send("reaction_added", "U1", "U2", "fire::skin-tone-2") // tier 4 now highest: +3 more
	send("reaction_added", "U1", "U2", "star")              // below 4: nothing
	send("reaction_added", "U1", "U2", "wastebasket")       // 4 - 5 = -1: -5
	send("reaction_removed", "U1", "U2", "wastebasket")     // back to 4: +5, silent
	send("reaction_added", "U2", "U2", "trophy")            // self-reaction: ignored
	send("reaction_added", "U1", "UBOT", "trophy")          // reacting to the bot: ignored
	send("reaction_added", "U1", "U2", "tada")              // not a tier emoji: ignored

	want := []string{"U2 ++", "U2 ++++", "U2 ------", "U2 ++++++"}
	if strings.Join(runs, ",") != strings.Join(want, ",") {
		t.Fatalf("karma runs = %v, want %v", runs, want)
	}
	// Only the three additions that changed the award reply, in the message's thread.
	if len(web.posts) != 3 {
		t.Fatalf("expected 3 thread replies, got %d", len(web.posts))
	}
	for _, post := range web.posts {
		if post.threadTS != "100.1" {
			t.Fatalf("reply should thread on the reacted message, got %q", post.threadTS)
		}
	}
}

type memOverrides struct{ channel, workspace domain.TierOverrides }

func (m *memOverrides) GetReactionOverrides(context.Context, string, string) (domain.TierOverrides, domain.TierOverrides, error) {
	return m.channel, m.workspace, nil
}

func (m *memOverrides) SetReactionOverrides(_ context.Context, _, _ string, workspace bool, _ string, o domain.TierOverrides) error {
	if workspace {
		m.workspace = o
	} else {
		m.channel = o
	}
	return nil
}

func TestReactionSettingsSetTierAndReset(t *testing.T) {
	store := &memOverrides{}
	s := NewReactionSettingsService(store)
	ctx := context.Background()

	if _, err := s.SetTier(ctx, "T1", "C1", true, "U1", "3", []string{":partyparrot:"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetTier(ctx, "T1", "C1", false, "U1", "5", []string{":partyparrot:"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetTier(ctx, "T1", "C1", false, "U1", "-2", []string{":partyparrot:", ":meh:"}); err != nil {
		t.Fatal(err)
	}
	tiers, _ := s.ReactionTiers(ctx, "T1", "C1")
	if tiers["partyparrot"] != -2 || tiers["meh"] != -2 {
		t.Fatalf("channel override should win and move the emoji, got %v", tiers)
	}
	if _, ok := store.channel[5]; ok {
		t.Fatalf("moving the only emoji out of a tier should drop the tier override, got %v", store.channel)
	}

	for _, bad := range [][]string{{"0", ":x:"}, {"6", ":x:"}, {"3"}, {"3", "<script>"}} {
		if _, err := s.SetTier(ctx, "T1", "C1", false, "U1", bad[0], bad[1:]); err == nil {
			t.Fatalf("SetTier(%v) should fail", bad)
		}
	}

	if _, err := s.Reset(ctx, "T1", "C1", false, "U1"); err != nil {
		t.Fatal(err)
	}
	tiers, _ = s.ReactionTiers(ctx, "T1", "C1")
	if tiers["partyparrot"] != 3 {
		t.Fatalf("after channel reset the workspace tier should apply, got %v", tiers)
	}
}

func TestSettingsEmojiCommandRouting(t *testing.T) {
	store := &memOverrides{}
	p := NewCommandsProcessor("secret", fakeLeaderboardService{}, fakeSettingsCommandService{}, "test").
		WithReactionSettings(NewReactionSettingsService(store))

	run := func(text string) string {
		body := []byte("team_id=T1&channel_id=C1&user_id=U1&command=%2Fplusplus-settings&text=" + text)
		req := httptest.NewRequest(http.MethodPost, "/slack/commands", bytes.NewReader(body))
		addSignedHeaders(req, "secret", body)
		rec := httptest.NewRecorder()
		p.ProcessCommand(rec, req)
		return rec.Body.String()
	}

	if out := run("emoji"); !strings.Contains(out, ":trophy:") {
		t.Fatalf("emoji should list the current map, got %s", out)
	}
	run("emoji+workspace+4+%3Ahotdog%3A")
	if store.workspace[4][0] != "hotdog" {
		t.Fatalf("workspace tier not saved: %v", store.workspace)
	}
	if out := run("emoji+9+%3Ax%3A"); !strings.Contains(out, "tier must be") {
		t.Fatalf("bad tier should explain, got %s", out)
	}
}
