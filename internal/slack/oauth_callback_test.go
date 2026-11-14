package slack

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func newTestOAuthHandler() *OAuthHandler {
	return mustOAuthHandler(NewOAuthHandler("client-id", "client-secret", "https://example.com", nil, "state-secret", ""))
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
	h := mustOAuthHandler(NewOAuthHandler("client-id", "client-secret", "https://example.com", installer, "state-secret", ""))
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

func mustOAuthHandler(h *OAuthHandler, err error) *OAuthHandler {
	if err != nil {
		panic(err)
	}
	return h
}

func TestNewOAuthHandlerRequiresAnAbsoluteBaseURL(t *testing.T) {
	for _, base := range []string{"", "/", "example.com", "https://"} {
		if _, err := NewOAuthHandler("client-id", "client-secret", base, nil, "state-secret", ""); err == nil {
			t.Errorf("base URL %q: want an error", base)
		}
	}
}

// Slack rejects any redirect_uri that differs from the app's configured one.
func TestInstallSendsTheConfiguredRedirectURI(t *testing.T) {
	h := mustOAuthHandler(NewOAuthHandler("client-id", "client-secret", "https://example.com/", nil, "state-secret", ""))
	w := httptest.NewRecorder()
	h.Install(w, httptest.NewRequest(http.MethodGet, "/slack/install", nil))
	loc, err := url.Parse(w.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if got := loc.Query().Get("redirect_uri"); got != "https://example.com/slack/oauth/callback" {
		t.Fatalf("redirect_uri = %q", got)
	}
}

// The redirect origin is fixed config, whatever headers the request carries.
func TestCallbackIgnoresForwardedHostHeaders(t *testing.T) {
	h := newTestOAuthHandler()
	req := httptest.NewRequest(http.MethodGet, "/slack/oauth/callback", nil)
	req.Header.Set("X-Forwarded-Host", "evil.example")
	req.Header.Set("X-Forwarded-Proto", "https")
	w := httptest.NewRecorder()
	h.Callback(w, req)
	if loc := w.Header().Get("Location"); loc != "https://example.com/slack/install" {
		t.Fatalf("redirected to %q, want the configured base URL", loc)
	}
}

func TestCallbackFailuresShowFixedMessages(t *testing.T) {
	t.Run("failed exchange", func(t *testing.T) {
		h := newTestOAuthHandler()
		h.httpClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": {"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"ok":false,"error":"invalid_code_upstream_detail"}`)),
			}, nil
		})}
		w := httptest.NewRecorder()
		h.Callback(w, httptest.NewRequest(http.MethodGet, "/slack/oauth/callback?code=c1", nil))
		if w.Code != http.StatusBadGateway || strings.Contains(w.Body.String(), "invalid_code_upstream_detail") {
			t.Fatalf("got %d %q; want 502 without the upstream error", w.Code, w.Body.String())
		}
	})
	for param, wantShown := range map[string]bool{"access_denied": true, "<script>alert(1)</script>": false} {
		t.Run("error="+param, func(t *testing.T) {
			w := httptest.NewRecorder()
			newTestOAuthHandler().Callback(w, httptest.NewRequest(http.MethodGet, "/slack/oauth/callback?error="+url.QueryEscape(param), nil))
			if w.Code != http.StatusBadRequest || strings.Contains(w.Body.String(), param) != wantShown {
				t.Fatalf("got %d %q; parameter shown = %v, want %v", w.Code, w.Body.String(), !wantShown, wantShown)
			}
		})
	}
}

// State is signed with a key derived from the secret, not the secret itself: a state minted
// the old way is rejected, and one minted by Install is accepted.
func TestCallbackStateUsesTheDerivedKey(t *testing.T) {
	installer := &fakeInstaller{}
	h := mustOAuthHandler(NewOAuthHandler("client-id", "client-secret", "https://example.com", installer, "state-secret", ""))
	h.httpClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": {"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"ok":true,"access_token":"xoxb-1","team":{"id":"T1"}}`)),
		}, nil
	})}

	oldScheme := mintOAuthStateAt("state-secret", time.Now())
	w := httptest.NewRecorder()
	h.Callback(w, httptest.NewRequest(http.MethodGet, "/slack/oauth/callback?code=c1&state="+oldScheme, nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("state signed with the raw secret: got %d, want 400", w.Code)
	}

	install := httptest.NewRecorder()
	h.Install(install, httptest.NewRequest(http.MethodGet, "/slack/install", nil))
	loc, _ := url.Parse(install.Header().Get("Location"))
	state := loc.Query().Get("state")
	w = httptest.NewRecorder()
	h.Callback(w, httptest.NewRequest(http.MethodGet, "/slack/oauth/callback?code=c1&state="+url.QueryEscape(state), nil))
	if w.Code != http.StatusOK || installer.teamID != "T1" {
		t.Fatalf("state minted by Install: got %d %q, installer %+v", w.Code, w.Body.String(), installer)
	}
}
