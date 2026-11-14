package slack

import (
	"bytes"
	"context"
	"errors"
	"maps"
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

// memReactionState mimics the DynamoDB store: per-row emoji -> tier, returning the old map.
type memReactionState map[string]map[string]int

func (m memReactionState) AddReaction(_ context.Context, _, ch, ts, who, emoji string, tier int) (map[string]int, error) {
	key := ch + ts + who
	before := maps.Clone(m[key])
	if m[key] == nil {
		m[key] = map[string]int{}
	}
	m[key][emoji] = tier
	return before, nil
}

func (m memReactionState) RemoveReaction(_ context.Context, _, ch, ts, who, emoji string) (map[string]int, error) {
	key := ch + ts + who
	if _, ok := m[key][emoji]; !ok {
		return nil, nil
	}
	before := maps.Clone(m[key])
	delete(m[key], emoji)
	return before, nil
}

// Karma message targets live under their own key; the int is the target's position.
func (m memReactionState) RecordKarmaTargets(_ context.Context, _, ch, ts string, targets []string) error {
	m["#km#"+ch+ts] = map[string]int{}
	for i, t := range targets {
		m["#km#"+ch+ts][t] = i
	}
	return nil
}

func (m memReactionState) KarmaTargets(_ context.Context, _, ch, ts string) ([]string, bool, error) {
	row, ok := m["#km#"+ch+ts]
	if !ok {
		return nil, false, nil
	}
	targets := make([]string, len(row))
	for t, i := range row {
		targets[i] = t
	}
	return targets, true, nil
}

type deltaRecorder struct {
	runs    *[]string
	skipWar *[]bool
	failN   *int // fail this many calls before succeeding
	failOn  *int // counts down calls; the call that takes it to 0 fails
}

func (d deltaRecorder) HandleAction(_ context.Context, a domain.KarmaAction) (domain.KarmaResult, error) {
	if d.failN != nil && *d.failN > 0 {
		*d.failN--
		return domain.KarmaResult{}, errors.New("dynamo down")
	}
	if d.failOn != nil {
		if *d.failOn--; *d.failOn == 0 {
			return domain.KarmaResult{}, errors.New("dynamo down")
		}
	}
	*d.runs = append(*d.runs, a.TargetUserID+" "+a.SymbolRun)
	if d.skipWar != nil {
		*d.skipWar = append(*d.skipWar, a.SkipWar)
	}
	return domain.KarmaResult{ShouldPersist: true, Message: "applied"}, nil
}

// failingStore makes the karma-message target calls fail.
type failingStore struct {
	memReactionState
	failRecord, failLookup bool
}

func (f failingStore) RecordKarmaTargets(ctx context.Context, team, ch, ts string, targets []string) error {
	if f.failRecord {
		return errors.New("dynamo down")
	}
	return f.memReactionState.RecordKarmaTargets(ctx, team, ch, ts, targets)
}

func (f failingStore) KarmaTargets(ctx context.Context, team, ch, ts string) ([]string, bool, error) {
	if f.failLookup {
		return nil, false, errors.New("dynamo down")
	}
	return f.memReactionState.KarmaTargets(ctx, team, ch, ts)
}

// postKarma sends a typed karma message from U2 at ts and returns the HTTP status.
func postKarma(p *EventsProcessor, ts, text string) int {
	body := []byte(`{"type":"event_callback","team_id":"T1",
		"authorizations":[{"user_id":"UBOT","is_bot":true}],
		"event":{"type":"app_mention","user":"U2","text":"<@UBOT> ` + text + `","channel":"C1","ts":"` + ts + `","event_ts":"` + ts + `"}}`)
	req := httptest.NewRequest(http.MethodPost, "/slack/events", bytes.NewReader(body))
	addSlackSignatureHeaders(req, "secret", body)
	rec := httptest.NewRecorder()
	p.ProcessEvent(rec, req)
	return rec.Code
}

// reactAt sends a reaction event on the message at ts (authored by U2) and returns the HTTP status.
func reactAt(p *EventsProcessor, kind, ts, reactor, emoji string, headers ...string) int {
	body := []byte(strings.Replace(string(reactionEvent(kind, reactor, "U2", emoji)), `"ts":"100.1"`, `"ts":"`+ts+`"`, 1))
	req := httptest.NewRequest(http.MethodPost, "/slack/events", bytes.NewReader(body))
	addSlackSignatureHeaders(req, "secret", body)
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	p.ProcessEvent(rec, req)
	return rec.Code
}

// mutableTiers lets a test remap emojis between events.
type mutableTiers struct{ m map[string]int }

func (t *mutableTiers) ReactionTiers(context.Context, string, string) (map[string]int, error) {
	return t.m, nil
}

type failingWeb struct{}

func (f *failingWeb) PostMessage(context.Context, string, string, string, string) error {
	return errors.New("rate_limited")
}

// sendReaction posts a signed reaction event and returns the HTTP status.
func sendReaction(t *testing.T, p *EventsProcessor, kind, reactor, author, emoji string, headers ...string) int {
	t.Helper()
	body := reactionEvent(kind, reactor, author, emoji)
	req := httptest.NewRequest(http.MethodPost, "/slack/events", bytes.NewReader(body))
	addSlackSignatureHeaders(req, "secret", body)
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	p.ProcessEvent(rec, req)
	return rec.Code
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
	p := NewEventsProcessor("secret", deltaRecorder{runs: &runs}, nil, nil, web).WithReactions(defaultTiers{}, memReactionState{}, 5)

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

func TestReactionsOnTypedKarmaCreditTheMentionedPeople(t *testing.T) {
	var runs []string
	web := &fakeWebClient{}
	p := NewEventsProcessor("secret", deltaRecorder{runs: &runs}, nil, nil, web).WithReactions(defaultTiers{}, memReactionState{}, 5)

	post := func(ts, text string) {
		if code := postKarma(p, ts, text); code != http.StatusOK {
			t.Fatalf("post %q: status %d", text, code)
		}
	}
	react := func(kind, ts, reactor, emoji string) {
		if code := reactAt(p, kind, ts, reactor, emoji); code != http.StatusOK {
			t.Fatalf("%s %s: status %d", kind, emoji, code)
		}
	}

	post("100.1", "<@U3> -- <@U4> --") // U2 docks U3 and U4
	post("100.2", "<!subteam^S1|@team> ++")
	runs, web.posts = nil, nil

	react("reaction_added", "100.1", "U1", "fire")   // emojis keep their meaning: +4 to each mentioned person
	react("reaction_added", "100.1", "U3", "clap")   // a mentioned person can't credit themselves: U4 only
	react("reaction_added", "100.1", "U2", "trophy") // the poster reacting to their own message: ignored
	react("reaction_added", "100.2", "U1", "fire")   // user-group award: credits nobody
	react("reaction_added", "100.9", "U1", "clap")   // ordinary message: the author, as before
	react("reaction_removed", "100.1", "U1", "fire") // removal takes it back from the same people
	react("reaction_removed", "100.1", "U3", "clap") // and still skips the reactor

	want := "U3 +++++,U4 +++++,U4 ++,U2 ++,U3 -----,U4 -----,U4 --"
	if got := strings.Join(runs, ","); got != want {
		t.Fatalf("karma runs = %s, want %s", got, want)
	}
	if len(web.posts) != 3 || web.posts[0].text != "applied\napplied" {
		t.Fatalf("expected one combined reply per award, got %+v", web.posts)
	}
}

func TestReactionPartialFailureReversesAndRetriesCleanly(t *testing.T) {
	var runs []string
	failOn := -1 // disarmed while the typed message is posted
	state := memReactionState{}
	p := NewEventsProcessor("secret", deltaRecorder{runs: &runs, failOn: &failOn}, nil, nil, &fakeWebClient{}).WithReactions(defaultTiers{}, state, 5)

	if code := postKarma(p, "100.1", "<@U3> ++ <@U4> ++"); code != http.StatusOK {
		t.Fatalf("post: status %d", code)
	}
	runs = nil
	failOn = 2 // now the reaction's second target fails

	if code := reactAt(p, "reaction_added", "100.1", "U1", "fire"); code != http.StatusInternalServerError {
		t.Fatalf("a failed award should ask Slack to retry, got %d", code)
	}
	if got := strings.Join(runs, ","); got != "U3 +++++,U3 -----" {
		t.Fatalf("the first target's karma should be reversed, got %s", got)
	}
	if len(state["C1100.1U1"]) != 0 {
		t.Fatalf("reaction state should be rolled back, got %v", state["C1100.1U1"])
	}

	runs = nil
	if code := reactAt(p, "reaction_added", "100.1", "U1", "fire", "X-Slack-Retry-Num", "1", "X-Slack-Retry-Reason", "http_error"); code != http.StatusOK {
		t.Fatalf("retry status %d", code)
	}
	reactAt(p, "reaction_removed", "100.1", "U1", "fire")
	if got := strings.Join(runs, ","); got != "U3 +++++,U4 +++++,U3 -----,U4 -----" {
		t.Fatalf("retry should credit everyone once and removal take it all back, got %s", got)
	}
}

func TestKarmaMessageStoreFailuresAreSafeToRetry(t *testing.T) {
	var runs []string
	p := NewEventsProcessor("secret", deltaRecorder{runs: &runs}, nil, nil, &fakeWebClient{}).
		WithReactions(defaultTiers{}, failingStore{memReactionState: memReactionState{}, failRecord: true}, 5)
	if code := postKarma(p, "100.1", "<@U3> ++"); code != http.StatusInternalServerError || len(runs) != 0 {
		t.Fatalf("a failed record must apply no karma and ask for a retry: code %d runs %v", code, runs)
	}

	state := memReactionState{}
	p = NewEventsProcessor("secret", deltaRecorder{runs: &runs}, nil, nil, &fakeWebClient{}).
		WithReactions(defaultTiers{}, failingStore{memReactionState: state, failLookup: true}, 5)
	if code := reactAt(p, "reaction_added", "100.1", "U1", "fire"); code != http.StatusInternalServerError || len(runs) != 0 {
		t.Fatalf("a failed lookup must apply no karma and ask for a retry: code %d runs %v", code, runs)
	}
	if len(state["C1100.1U1"]) != 0 {
		t.Fatalf("reaction state should be rolled back, got %v", state["C1100.1U1"])
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

func TestReactionUndoUsesStoredTierAfterRemap(t *testing.T) {
	var runs []string
	tiers := &mutableTiers{m: domain.ResolveReactionTiers()}
	p := NewEventsProcessor("secret", deltaRecorder{runs: &runs}, nil, nil, &fakeWebClient{}).WithReactions(tiers, memReactionState{}, 5)

	sendReaction(t, p, "reaction_added", "U1", "U2", "fire") // +4
	tiers.m = domain.ResolveReactionTiers(domain.TierOverrides{-5: {"fire"}})
	sendReaction(t, p, "reaction_removed", "U1", "U2", "fire")                 // must undo the +4, not "remove -5"
	tiers.m = domain.ResolveReactionTiers(domain.TierOverrides{4: {"rocket"}}) // fire unmapped
	sendReaction(t, p, "reaction_added", "U1", "U2", "fire")                   // ignored now

	if strings.Join(runs, ",") != "U2 +++++,U2 -----" {
		t.Fatalf("remap must not mint karma, got %v", runs)
	}
}

func TestReactionDeltasStayWithinLowCap(t *testing.T) {
	var runs []string
	p := NewEventsProcessor("secret", deltaRecorder{runs: &runs}, nil, nil, &fakeWebClient{}).WithReactions(defaultTiers{}, memReactionState{}, 3)

	sendReaction(t, p, "reaction_added", "U1", "U2", "trophy")   // clamped to +3
	sendReaction(t, p, "reaction_added", "U1", "U2", "clap")     // below 3: nothing
	sendReaction(t, p, "reaction_removed", "U1", "U2", "trophy") // 3 -> 1: -2
	sendReaction(t, p, "reaction_removed", "U1", "U2", "clap")   // 1 -> 0: -1

	net := 0
	for _, r := range runs {
		n := len(strings.Fields(r)[1]) - 1
		if n > 3 {
			t.Fatalf("delta %d exceeds the cap: %v", n, runs)
		}
		if strings.Contains(r, "-") {
			n = -n
		}
		net += n
	}
	if net != 0 {
		t.Fatalf("add then remove must net to zero, got %d from %v", net, runs)
	}
}

func TestReactionStateRestoredWhenKarmaFails(t *testing.T) {
	var runs []string
	fail := 1
	state := memReactionState{}
	p := NewEventsProcessor("secret", deltaRecorder{runs: &runs, failN: &fail}, nil, nil, &fakeWebClient{}).WithReactions(defaultTiers{}, state, 5)

	if code := sendReaction(t, p, "reaction_added", "U1", "U2", "fire"); code != http.StatusInternalServerError {
		t.Fatalf("karma failure should ask Slack to retry, got %d", code)
	}
	if len(state["C1100.1U1"]) != 0 {
		t.Fatalf("state should be rolled back, got %v", state)
	}
	if code := sendReaction(t, p, "reaction_added", "U1", "U2", "fire", "X-Slack-Retry-Num", "1", "X-Slack-Retry-Reason", "http_error"); code != http.StatusOK {
		t.Fatalf("retry status %d", code)
	}
	if strings.Join(runs, ",") != "U2 +++++" {
		t.Fatalf("retry should apply the karma once, got %v", runs)
	}
}

func TestReactionTimeoutRetryIsIgnored(t *testing.T) {
	var runs []string
	p := NewEventsProcessor("secret", deltaRecorder{runs: &runs}, nil, nil, &fakeWebClient{}).WithReactions(defaultTiers{}, memReactionState{}, 5)
	if code := sendReaction(t, p, "reaction_added", "U1", "U2", "fire", "X-Slack-Retry-Num", "1", "X-Slack-Retry-Reason", "http_timeout"); code != http.StatusOK || len(runs) != 0 {
		t.Fatalf("timeout retry must be acked without reprocessing: code %d runs %v", code, runs)
	}
}

func TestReactionReplyFailureKeepsKarmaAndAcks(t *testing.T) {
	var runs []string
	p := NewEventsProcessor("secret", deltaRecorder{runs: &runs}, nil, nil, &failingWeb{}).WithReactions(defaultTiers{}, memReactionState{}, 5)
	if code := sendReaction(t, p, "reaction_added", "U1", "U2", "fire"); code != http.StatusOK || len(runs) != 1 {
		t.Fatalf("a failed reply should not fail the event: code %d runs %v", code, runs)
	}
}

func TestReactionUpgradesAndRemovalsSkipKarmaWar(t *testing.T) {
	var runs []string
	var skip []bool
	p := NewEventsProcessor("secret", deltaRecorder{runs: &runs, skipWar: &skip}, nil, nil, &fakeWebClient{}).WithReactions(defaultTiers{}, memReactionState{}, 5)
	sendReaction(t, p, "reaction_added", "U1", "U2", "clap")     // first hit: counts for war detection
	sendReaction(t, p, "reaction_added", "U1", "U2", "trophy")   // upgrade
	sendReaction(t, p, "reaction_removed", "U1", "U2", "trophy") // removal
	if len(skip) != 3 || skip[0] || !skip[1] || !skip[2] {
		t.Fatalf("SkipWar = %v, want [false true true]", skip)
	}
}

type fakeAdmins map[string]bool

func (f fakeAdmins) IsWorkspaceAdmin(_ context.Context, _, user string) (bool, error) {
	return f[user], nil
}

func TestWorkspaceEmojiChangesNeedAdmin(t *testing.T) {
	store := &memOverrides{}
	s := NewReactionSettingsService(store).WithAdminCheck(fakeAdmins{"UADMIN": true})
	ctx := context.Background()

	var input *InputError
	if _, err := s.SetTier(ctx, "T1", "C1", true, "UGUEST", "5", []string{":x:"}); !errors.As(err, &input) {
		t.Fatalf("non-admin workspace edit should be refused, got %v", err)
	}
	if _, err := s.Reset(ctx, "T1", "C1", true, "UGUEST"); !errors.As(err, &input) {
		t.Fatalf("non-admin workspace reset should be refused, got %v", err)
	}
	if _, err := s.SetTier(ctx, "T1", "C1", false, "UGUEST", "5", []string{":x:"}); err != nil {
		t.Fatalf("anyone can change a channel's emojis: %v", err)
	}
	if _, err := s.SetTier(ctx, "T1", "C1", true, "UADMIN", "5", []string{":x:"}); err != nil {
		t.Fatalf("admin workspace edit: %v", err)
	}
}

type brokenOverrides struct{ memOverrides }

func (b *brokenOverrides) SetReactionOverrides(context.Context, string, string, bool, string, domain.TierOverrides) error {
	return errors.New("ProvisionedThroughputExceededException: internal detail")
}

func TestEmojiCommandBranchesAndErrors(t *testing.T) {
	run := func(p *CommandsProcessor, text string) string {
		body := []byte("team_id=T1&channel_id=C1&user_id=U1&command=%2Fplusplus-settings&text=" + text)
		req := httptest.NewRequest(http.MethodPost, "/slack/commands", bytes.NewReader(body))
		addSignedHeaders(req, "secret", body)
		rec := httptest.NewRecorder()
		p.ProcessCommand(rec, req)
		return rec.Body.String()
	}
	newP := func(store ReactionOverridesStore) *CommandsProcessor {
		return NewCommandsProcessor("secret", fakeLeaderboardService{}, fakeSettingsCommandService{}, "test").
			WithReactionSettings(NewReactionSettingsService(store))
	}

	store := &memOverrides{workspace: domain.TierOverrides{3: {"x"}}}
	p := newP(store)
	if out := run(p, "emoji+workspace+reset"); !strings.Contains(out, "reset") || len(store.workspace) != 0 {
		t.Fatalf("workspace reset: %s %v", out, store.workspace)
	}
	if out := run(p, "emoji+workspace"); !strings.Contains(out, "PlusPlus help") {
		t.Fatalf("bare `emoji workspace` should show usage, got %s", out)
	}

	broken := newP(&brokenOverrides{})
	out := run(broken, "emoji+3+%3Ax%3A")
	if strings.Contains(out, "Provisioned") || strings.Contains(out, "Example") || !strings.Contains(out, "try again") {
		t.Fatalf("storage errors should be generic, got %s", out)
	}

	noReactions := NewCommandsProcessor("secret", fakeLeaderboardService{}, fakeSettingsCommandService{}, "test")
	if out := run(noReactions, "emoji"); !strings.Contains(out, "PlusPlus help") {
		t.Fatalf("emoji without reaction support should show usage, got %s", out)
	}
}

func TestSettingsPanelShowsKarmaReactions(t *testing.T) {
	run := func(p *CommandsProcessor) string {
		body := []byte("team_id=T1&channel_id=C1&user_id=U1&command=%2Fplusplus-settings&text=")
		req := httptest.NewRequest(http.MethodPost, "/slack/commands", bytes.NewReader(body))
		addSignedHeaders(req, "secret", body)
		rec := httptest.NewRecorder()
		p.ProcessCommand(rec, req)
		return rec.Body.String()
	}
	base := func() *CommandsProcessor {
		return NewCommandsProcessor("secret", fakeLeaderboardService{}, fakeSettingsCommandService{}, "test")
	}

	withReactions := run(base().WithReactionSettings(NewReactionSettingsService(&memOverrides{channel: domain.TierOverrides{3: {"partyparrot"}}})))
	for _, want := range []string{"snark_level_select", "Karma reactions", ":partyparrot:", ":trophy:", "/plusplus-settings emoji"} {
		if !strings.Contains(withReactions, want) {
			t.Fatalf("settings panel should include %q, got %s", want, withReactions)
		}
	}

	broken := run(base().WithReactionSettings(NewReactionSettingsService(&failingGetOverrides{})))
	if !strings.Contains(broken, "snark_level_select") || strings.Contains(broken, "Karma reactions") {
		t.Fatalf("panel should still render without the emoji section when it can't load, got %s", broken)
	}
}

type failingGetOverrides struct{ memOverrides }

func (f *failingGetOverrides) GetReactionOverrides(context.Context, string, string) (domain.TierOverrides, domain.TierOverrides, error) {
	return nil, nil, errors.New("dynamo down")
}
