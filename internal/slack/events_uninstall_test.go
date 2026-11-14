package slack

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

type recordingRemover struct {
	teams []string
	times []time.Time
	err   error
}

func (r *recordingRemover) RemoveInstallation(_ context.Context, teamID string, uninstalledAt time.Time) error {
	r.teams = append(r.teams, teamID)
	r.times = append(r.times, uninstalledAt)
	return r.err
}

func sendEvent(p *EventsProcessor, event string, sign bool) int {
	body := []byte(`{"type":"event_callback","team_id":"T1","event_time":1700000000,"event":` + event + `}`)
	req := httptest.NewRequest(http.MethodPost, "/slack/events", bytes.NewReader(body))
	if sign {
		addSlackSignatureHeaders(req, "secret", body)
	}
	rec := httptest.NewRecorder()
	serveEvent(p, rec, req)
	return rec.Code
}

func TestUninstallEventsRemoveTheBotToken(t *testing.T) {
	cases := map[string]struct {
		event       string
		sign        bool
		storeErr    error
		wantStatus  int
		wantRemoved []string
	}{
		"app uninstalled":          {event: `{"type":"app_uninstalled"}`, sign: true, wantStatus: http.StatusOK, wantRemoved: []string{"T1"}},
		"bot token revoked":        {event: `{"type":"tokens_revoked","tokens":{"bot":["UBOT"]}}`, sign: true, wantStatus: http.StatusOK, wantRemoved: []string{"T1"}},
		"only user tokens revoked": {event: `{"type":"tokens_revoked","tokens":{"oauth":["U1"]}}`, sign: true, wantStatus: http.StatusOK},
		"empty bot list":           {event: `{"type":"tokens_revoked","tokens":{"oauth":["U1"],"bot":[]}}`, sign: true, wantStatus: http.StatusOK},
		"user and bot revoked":     {event: `{"type":"tokens_revoked","tokens":{"oauth":["U1"],"bot":["UBOT"]}}`, sign: true, wantStatus: http.StatusOK, wantRemoved: []string{"T1"}},
		"unsigned request":         {event: `{"type":"app_uninstalled"}`, sign: false, wantStatus: http.StatusUnauthorized},
		"storage error is retried": {event: `{"type":"app_uninstalled"}`, sign: true, storeErr: errors.New("dynamo down"), wantStatus: http.StatusInternalServerError, wantRemoved: []string{"T1"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			remover := &recordingRemover{err: tc.storeErr}
			p := NewEventsProcessor(fakeKarmaActionService{}, nil, nil, &fakeWebClient{}).WithInstallationRemover(remover)
			if got := sendEvent(p, tc.event, tc.sign); got != tc.wantStatus {
				t.Fatalf("status %d, want %d", got, tc.wantStatus)
			}
			if !reflect.DeepEqual(remover.teams, tc.wantRemoved) {
				t.Fatalf("removed %v, want %v", remover.teams, tc.wantRemoved)
			}
			for _, at := range remover.times {
				if !at.Equal(time.Unix(1700000000, 0)) {
					t.Fatalf("removal cutoff %v, want the event's event_time", at)
				}
			}
		})
	}
}

// Single-workspace mode has no token store; the events are acknowledged and ignored.
func TestUninstallWithoutStoreIsAcknowledged(t *testing.T) {
	p := NewEventsProcessor(fakeKarmaActionService{}, nil, nil, &fakeWebClient{})
	if got := sendEvent(p, `{"type":"app_uninstalled"}`, true); got != http.StatusOK {
		t.Fatalf("status %d, want 200", got)
	}
}

// Uninstall events are not karma: they never claim an event or apply karma.
func TestUninstallSkipsKarmaHandling(t *testing.T) {
	var calls []string
	claims := newMemClaims()
	p := NewEventsProcessor(countingKarma{calls: &calls}, nil, nil, &fakeWebClient{}).
		WithEventClaims(claims).
		WithInstallationRemover(&recordingRemover{})
	if got := sendEvent(p, `{"type":"app_uninstalled"}`, true); got != http.StatusOK {
		t.Fatalf("status %d", got)
	}
	if len(calls) != 0 || len(claims.held) != 0 {
		t.Fatalf("uninstall touched karma: calls %v, claims %v", calls, claims.held)
	}
}
