package slack

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

type recordingRemover struct {
	teams []string
	err   error
}

func (r *recordingRemover) RemoveInstallation(_ context.Context, teamID string) error {
	r.teams = append(r.teams, teamID)
	return r.err
}

func sendEvent(p *EventsProcessor, event string, sign bool) int {
	body := []byte(`{"type":"event_callback","team_id":"T1","event":` + event + `}`)
	req := httptest.NewRequest(http.MethodPost, "/slack/events", bytes.NewReader(body))
	if sign {
		addSlackSignatureHeaders(req, "secret", body)
	}
	rec := httptest.NewRecorder()
	p.ProcessEvent(rec, req)
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
		"unsigned request":         {event: `{"type":"app_uninstalled"}`, sign: false, wantStatus: http.StatusUnauthorized},
		"storage error is retried": {event: `{"type":"app_uninstalled"}`, sign: true, storeErr: errors.New("dynamo down"), wantStatus: http.StatusInternalServerError, wantRemoved: []string{"T1"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			remover := &recordingRemover{err: tc.storeErr}
			p := NewEventsProcessor("secret", fakeKarmaActionService{}, nil, nil, &fakeWebClient{}).WithInstallationRemover(remover)
			if got := sendEvent(p, tc.event, tc.sign); got != tc.wantStatus {
				t.Fatalf("status %d, want %d", got, tc.wantStatus)
			}
			if !reflect.DeepEqual(remover.teams, tc.wantRemoved) {
				t.Fatalf("removed %v, want %v", remover.teams, tc.wantRemoved)
			}
		})
	}
}

// Single-workspace mode has no token store; the events are acknowledged and ignored.
func TestUninstallWithoutStoreIsAcknowledged(t *testing.T) {
	p := NewEventsProcessor("secret", fakeKarmaActionService{}, nil, nil, &fakeWebClient{})
	if got := sendEvent(p, `{"type":"app_uninstalled"}`, true); got != http.StatusOK {
		t.Fatalf("status %d, want 200", got)
	}
}
