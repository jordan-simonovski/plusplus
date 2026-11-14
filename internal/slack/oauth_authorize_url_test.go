package slack

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestOAuthAuthorizeQueryOmitsUserScopeWhenUnused(t *testing.T) {
	// Install must not send user_scope unless needed; empty user_scope has confused Slack OAuth.
	q := url.Values{}
	q.Set("client_id", "C")
	q.Set("scope", DefaultOAuthScopes)
	q.Set("redirect_uri", "https://example.com/slack/oauth/callback")
	q.Set("state", "signed-state-value")
	enc := q.Encode()
	if !strings.Contains(enc, "state=signed-state-value") {
		t.Fatalf("expected state param: %q", enc)
	}
	if strings.Contains(enc, "user_scope") {
		t.Fatalf("do not send user_scope for bot-only install: %q", enc)
	}
}

func TestInstallRequestsConfiguredScopes(t *testing.T) {
	for scopes, want := range map[string]string{"": DefaultOAuthScopes, "a:read,b:write": "a:read,b:write"} {
		h := mustOAuthHandler(NewOAuthHandler("client-id", "secret", "https://example.com", nil, "state-secret", scopes))
		w := httptest.NewRecorder()
		h.Install(w, httptest.NewRequest(http.MethodGet, "/slack/install", nil))
		loc, err := url.Parse(w.Header().Get("Location"))
		if err != nil || loc.Query().Get("scope") != want {
			t.Fatalf("scopes %q: authorize URL scope = %q, want %q", scopes, loc.Query().Get("scope"), want)
		}
	}
}
