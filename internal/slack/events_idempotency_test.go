package slack

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"plusplus/internal/domain"
)

// memClaims is an in-memory EventClaimer.
type memClaims struct {
	held       map[string]bool
	err        error
	releaseErr error
	releases   int
}

func newMemClaims() *memClaims { return &memClaims{held: map[string]bool{}} }

func (m *memClaims) ClaimEvent(ctx context.Context, team, event string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if m.err != nil {
		return false, m.err
	}
	if m.held[team+"/"+event] {
		return false, nil
	}
	m.held[team+"/"+event] = true
	return true, nil
}

func (m *memClaims) ReleaseEvent(ctx context.Context, team, event string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if m.releaseErr != nil {
		return m.releaseErr
	}
	delete(m.held, team+"/"+event)
	m.releases++
	return nil
}

// countingKarma records every HandleAction target and fails for the targets in fail.
type countingKarma struct {
	calls *[]string
	fail  map[string]bool
}

func (c countingKarma) HandleAction(ctx context.Context, a domain.KarmaAction) (domain.KarmaResult, error) {
	*c.calls = append(*c.calls, a.TargetUserID)
	if err := ctx.Err(); err != nil {
		return domain.KarmaResult{}, err
	}
	if c.fail[a.TargetUserID] {
		return domain.KarmaResult{}, errors.New("dynamo down")
	}
	return domain.KarmaResult{ShouldPersist: true, Message: "applied"}, nil
}

// deliver sends a signed app_mention from U2 with the given event_id ("" omits it).
func deliver(ctx context.Context, p *EventsProcessor, eventID, text string) int {
	idField := ""
	if eventID != "" {
		idField = `"event_id":"` + eventID + `",`
	}
	body := []byte(`{"type":"event_callback","team_id":"T1",` + idField + `
		"authorizations":[{"user_id":"UBOT","is_bot":true}],
		"event":{"type":"app_mention","user":"U2","text":"<@UBOT> ` + text + `","channel":"C1","ts":"5.1","event_ts":"5.1"}}`)
	req := httptest.NewRequest(http.MethodPost, "/slack/events", bytes.NewReader(body)).WithContext(ctx)
	addSlackSignatureHeaders(req, "secret", body)
	rec := httptest.NewRecorder()
	p.ProcessEvent(rec, req)
	return rec.Code
}

func TestTypedKarmaRetryAppliesOnce(t *testing.T) {
	var calls []string
	web := &fakeWebClient{}
	p := NewEventsProcessor("secret", countingKarma{calls: &calls}, nil, nil, web).WithEventClaims(newMemClaims())

	for i, want := range []int{http.StatusOK, http.StatusOK} {
		if got := deliver(context.Background(), p, "Ev1", "<@U3> ++"); got != want {
			t.Fatalf("delivery %d: status %d, want %d", i+1, got, want)
		}
	}
	if !reflect.DeepEqual(calls, []string{"U3"}) || len(web.posts) != 1 {
		t.Fatalf("karma calls %v and %d replies; want one of each", calls, len(web.posts))
	}
}

func TestTypedKarmaClaimErrorAppliesNothing(t *testing.T) {
	var calls []string
	claims := newMemClaims()
	claims.err = errors.New("dynamo down")
	p := NewEventsProcessor("secret", countingKarma{calls: &calls}, nil, nil, &fakeWebClient{}).WithEventClaims(claims)

	if got := deliver(context.Background(), p, "Ev1", "<@U3> ++"); got != http.StatusInternalServerError {
		t.Fatalf("status %d, want 500", got)
	}
	if len(calls) != 0 {
		t.Fatalf("karma applied despite claim failure: %v", calls)
	}
}

func TestTypedKarmaRecordFailureReleasesClaim(t *testing.T) {
	var calls []string
	claims := newMemClaims()
	karma := countingKarma{calls: &calls}

	failing := NewEventsProcessor("secret", karma, nil, nil, &fakeWebClient{}).
		WithReactions(defaultTiers{}, failingStore{memReactionState: memReactionState{}, failRecord: true}, 5).
		WithEventClaims(claims)
	if got := deliver(context.Background(), failing, "Ev1", "<@U3> ++"); got != http.StatusInternalServerError {
		t.Fatalf("first delivery: status %d, want 500", got)
	}
	if len(calls) != 0 || claims.releases != 1 {
		t.Fatalf("after record failure: calls %v, releases %d; want none and 1", calls, claims.releases)
	}

	// Slack's retry, once storage is back, is processed.
	healthy := NewEventsProcessor("secret", karma, nil, nil, &fakeWebClient{}).
		WithReactions(defaultTiers{}, memReactionState{}, 5).
		WithEventClaims(claims)
	if got := deliver(context.Background(), healthy, "Ev1", "<@U3> ++"); got != http.StatusOK {
		t.Fatalf("retry: status %d, want 200", got)
	}
	if !reflect.DeepEqual(calls, []string{"U3"}) {
		t.Fatalf("retry karma calls %v, want [U3]", calls)
	}
}

// Once HandleAction has been called karma may be written, so the claim is kept and a
// retry applies nothing.
func TestTypedKarmaKeepsClaimAfterKarmaStarts(t *testing.T) {
	cases := map[string]struct {
		fail      map[string]bool
		web       WebClient
		wantFirst int
		wantCalls []string
	}{
		"first target fails":  {fail: map[string]bool{"U3": true}, web: &fakeWebClient{}, wantFirst: http.StatusInternalServerError, wantCalls: []string{"U3"}},
		"second target fails": {fail: map[string]bool{"U4": true}, web: &fakeWebClient{}, wantFirst: http.StatusInternalServerError, wantCalls: []string{"U3", "U4"}},
		"reply fails":         {web: &failingWeb{}, wantFirst: http.StatusBadGateway, wantCalls: []string{"U3"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var calls []string
			claims := newMemClaims()
			p := NewEventsProcessor("secret", countingKarma{calls: &calls, fail: tc.fail}, nil, nil, tc.web).WithEventClaims(claims)

			if got := deliver(context.Background(), p, "Ev1", "<@U3> ++ <@U4> ++"); got != tc.wantFirst {
				t.Fatalf("first delivery: status %d, want %d", got, tc.wantFirst)
			}
			if !reflect.DeepEqual(calls, tc.wantCalls) {
				t.Fatalf("first delivery calls %v, want %v", calls, tc.wantCalls)
			}
			if got := deliver(context.Background(), p, "Ev1", "<@U3> ++ <@U4> ++"); got != http.StatusOK {
				t.Fatalf("retry: status %d, want 200", got)
			}
			if !reflect.DeepEqual(calls, tc.wantCalls) || claims.releases != 0 {
				t.Fatalf("retry applied karma or released the claim: calls %v, releases %d", calls, claims.releases)
			}
		})
	}
}

func TestTypedKarmaWithoutEventIDIsProcessed(t *testing.T) {
	var calls []string
	p := NewEventsProcessor("secret", countingKarma{calls: &calls}, nil, nil, &fakeWebClient{}).WithEventClaims(newMemClaims())

	for range 2 {
		if got := deliver(context.Background(), p, "", "<@U3> ++"); got != http.StatusOK {
			t.Fatalf("status %d, want 200", got)
		}
	}
	if !reflect.DeepEqual(calls, []string{"U3", "U3"}) {
		t.Fatalf("calls %v; without an event_id every delivery is processed", calls)
	}
}

// ctxWeb fails a post whose context is already done, like a real HTTP call would.
type ctxWeb struct{ fakeWebClient }

func (c *ctxWeb) PostMessage(ctx context.Context, team, channel, text, threadTS string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return c.fakeWebClient.PostMessage(ctx, team, channel, text, threadTS)
}

// Slack hangs up at three seconds; the claim, the karma and the reply must not be
// cancelled with the request.
func TestTypedKarmaSurvivesRequestCancellation(t *testing.T) {
	var calls []string
	web := &ctxWeb{}
	p := NewEventsProcessor("secret", countingKarma{calls: &calls}, nil, nil, web).WithEventClaims(newMemClaims())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := deliver(ctx, p, "Ev1", "<@U3> ++"); got != http.StatusOK {
		t.Fatalf("status %d, want 200 (the work must not see the cancelled request context)", got)
	}
	if len(calls) != 1 || len(web.posts) != 1 {
		t.Fatalf("calls %v, replies %d; want one of each", calls, len(web.posts))
	}
}

// If the release itself fails the event stays claimed: dropped, never doubled.
func TestTypedKarmaReleaseFailureKeepsClaim(t *testing.T) {
	var calls []string
	claims := newMemClaims()
	claims.releaseErr = errors.New("dynamo down")
	karma := countingKarma{calls: &calls}

	failing := NewEventsProcessor("secret", karma, nil, nil, &fakeWebClient{}).
		WithReactions(defaultTiers{}, failingStore{memReactionState: memReactionState{}, failRecord: true}, 5).
		WithEventClaims(claims)
	if got := deliver(context.Background(), failing, "Ev1", "<@U3> ++"); got != http.StatusInternalServerError {
		t.Fatalf("status %d, want the original 500", got)
	}

	healthy := NewEventsProcessor("secret", karma, nil, nil, &fakeWebClient{}).
		WithReactions(defaultTiers{}, memReactionState{}, 5).
		WithEventClaims(claims)
	if got := deliver(context.Background(), healthy, "Ev1", "<@U3> ++"); got != http.StatusOK || len(calls) != 0 {
		t.Fatalf("retry after failed release: status %d, calls %v; want 200 and nothing applied", got, calls)
	}
}

// A user-group award that fails partway keeps the claim and tells the channel who was
// credited, so nobody retypes it and doubles the first members' karma.
func TestTypedKarmaGroupAwardFailingPartway(t *testing.T) {
	var calls []string
	web := &fakeWebClient{}
	groups := &fakeUserGroupLister{ids: []string{"U3", "U4", "U5"}}
	p := NewEventsProcessor("secret", countingKarma{calls: &calls, fail: map[string]bool{"U4": true}}, nil, groups, web).
		WithEventClaims(newMemClaims())

	if got := deliver(context.Background(), p, "Ev1", "<!subteam^S1|@team> ++"); got != http.StatusBadGateway {
		t.Fatalf("first delivery: status %d, want 502", got)
	}
	if !reflect.DeepEqual(calls, []string{"U3", "U4"}) {
		t.Fatalf("calls %v, want [U3 U4]", calls)
	}
	if len(web.posts) != 1 || !strings.Contains(web.posts[0].text, "applied") || !strings.HasSuffix(web.posts[0].text, domain.GroupAwardStoppedNote) {
		t.Fatalf("replies %+v; want one listing the credited member and ending with the stopped note", web.posts)
	}

	if got := deliver(context.Background(), p, "Ev1", "<!subteam^S1|@team> ++"); got != http.StatusOK || len(calls) != 2 {
		t.Fatalf("retry: status %d, calls %v; want 200 and nothing new applied", got, calls)
	}
}
