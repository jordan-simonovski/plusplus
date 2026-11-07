package slack

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func newTestOAuthHandler() *OAuthHandler {
	return NewOAuthHandler("client-id", "client-secret", "https://example.com", nil, "state-secret", "")
}

func TestCallbackWithoutStateOrCodeRedirectsToInstall(t *testing.T) {
	h := newTestOAuthHandler()
	req := httptest.NewRequest(http.MethodGet, "/slack/oauth/callback", nil)
	w := httptest.NewRecorder()

	h.Callback(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "https://example.com/slack/install" {
		t.Fatalf("expected redirect to install endpoint, got %q", loc)
	}
}

type fakeInstaller struct{ teamID, token string }

func (f *fakeInstaller) UpsertInstallation(_ context.Context, teamID, botToken string) error {
	f.teamID, f.token = teamID, botToken
	return nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// Slack-initiated installs (Marketplace, shareable URL) arrive with a code but no state.
// They must complete in one pass, not bounce through a second OAuth.
func TestCallbackWithoutStateExchangesCodeInOnePass(t *testing.T) {
	installer := &fakeInstaller{}
	h := NewOAuthHandler("client-id", "client-secret", "https://example.com", installer, "state-secret", "")
	var form url.Values
	h.httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(r.Body)
		form, _ = url.ParseQuery(string(body))
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": {"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"ok":true,"access_token":"xoxb-1","team":{"id":"T1"}}`)),
		}, nil
	})}

	req := httptest.NewRequest(http.MethodGet, "/slack/oauth/callback?code=c1", nil)
	w := httptest.NewRecorder()
	h.Callback(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 success page, got %d: %s", w.Code, w.Body.String())
	}
	if installer.teamID != "T1" || installer.token != "xoxb-1" {
		t.Fatalf("installation not saved: %+v", installer)
	}
	if form.Get("code") != "c1" || form.Has("redirect_uri") {
		t.Fatalf("expected code exchanged without redirect_uri, got %v", form)
	}
}

func TestCallbackTamperedStateFailsClosed(t *testing.T) {
	h := newTestOAuthHandler()
	req := httptest.NewRequest(http.MethodGet, "/slack/oauth/callback?state=bogus&code=c", nil)
	w := httptest.NewRecorder()

	h.Callback(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for tampered state, got %d", w.Code)
	}
}
