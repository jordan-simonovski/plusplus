package slack

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestOAuthHandler() *OAuthHandler {
	return NewOAuthHandler("client-id", "client-secret", "https://example.com", nil, "state-secret")
}

func TestCallbackEmptyStateRedirectsToInstall(t *testing.T) {
	h := newTestOAuthHandler()
	// Generic install button: code present, no state. Slack also returns no code on the
	// authorize page redirect; include one to prove it is not exchanged.
	req := httptest.NewRequest(http.MethodGet, "/slack/oauth/callback?code=attacker-code", nil)
	w := httptest.NewRecorder()

	h.Callback(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "https://example.com/slack/install" {
		t.Fatalf("expected redirect to install endpoint, got %q", loc)
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
