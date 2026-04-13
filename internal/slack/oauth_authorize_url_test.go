package slack

import (
	"net/url"
	"strings"
	"testing"
)

func TestOAuthAuthorizeQueryOmitsUserScopeWhenUnused(t *testing.T) {
	// Install must not send user_scope unless needed; empty user_scope has confused Slack OAuth.
	q := url.Values{}
	q.Set("client_id", "C")
	q.Set("scope", oauthScopes)
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
