package slack

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// captureLogs sends slog output to a buffer for the rest of the test. slog.SetDefault
// also redirects the standard log package, so that is restored too.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev, prevOut, prevFlags := slog.Default(), log.Writer(), log.Flags()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() {
		slog.SetDefault(prev)
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
	})
	return &buf
}

// A 5xx must leave exactly one log line an operator can act on: which workspace, which
// event, why. The message text is user content and must never be logged.
func TestKarmaFailureIsLoggedWithoutMessageText(t *testing.T) {
	buf := captureLogs(t)

	var calls []string
	p := NewEventsProcessor(countingKarma{calls: &calls, fail: map[string]bool{"U3": true}}, nil, nil, &fakeWebClient{})
	if got := deliver(context.Background(), p, "Ev1", "<@U3> ++ for the secret launch plan"); got != http.StatusInternalServerError {
		t.Fatalf("status %d, want 500", got)
	}

	out := buf.String()
	for _, want := range []string{`"team":"T1"`, `"kind":"app_mention"`, `"err":"dynamo down"`} {
		if !strings.Contains(out, want) {
			t.Errorf("log %q is missing %s", out, want)
		}
	}
	if n := strings.Count(out, `"level":"ERROR"`); n != 1 {
		t.Errorf("got %d error lines, want exactly 1: %q", n, out)
	}
	if strings.Contains(out, "secret launch plan") {
		t.Errorf("log contains the message text: %q", out)
	}
}

// OAuth failures are logged, but never with the code, the client secret or a token.
func TestOAuthFailureLogsNoSecrets(t *testing.T) {
	cases := map[string]struct {
		slackBody string
		installer WorkspaceInstaller
		wantErr   string
	}{
		"exchange rejected": {slackBody: `{"ok":false,"error":"invalid_code"}`, installer: &fakeInstaller{}, wantErr: "invalid_code"},
		"save failed":       {slackBody: `{"ok":true,"access_token":"xoxb-live-token","team":{"id":"T1"}}`, installer: failingInstaller{}, wantErr: "dynamo down"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			buf := captureLogs(t)
			h := mustOAuthHandler(NewOAuthHandler("client-id", "client-secret-value", "https://example.com", tc.installer, "state-secret", ""))
			h.httpClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": {"application/json"}},
					Body:       io.NopCloser(strings.NewReader(tc.slackBody)),
				}, nil
			})}

			h.Callback(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/slack/oauth/callback?code=one-time-code", nil))

			out := buf.String()
			if !strings.Contains(out, `"kind":"oauth"`) || !strings.Contains(out, tc.wantErr) {
				t.Errorf("log %q should record the oauth failure (%s)", out, tc.wantErr)
			}
			for _, secret := range []string{"one-time-code", "client-secret-value", "xoxb-live-token"} {
				if strings.Contains(out, secret) {
					t.Errorf("log contains %q: %q", secret, out)
				}
			}
		})
	}
}

type failingInstaller struct{}

func (failingInstaller) UpsertInstallation(context.Context, string, string) error {
	return errors.New("dynamo down")
}
