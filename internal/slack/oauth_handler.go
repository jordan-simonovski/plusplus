package slack

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultOAuthScopes is the Marketplace scope set: karma comes from @plusplus mentions and
// emoji reactions, with no access to message history. Self-hosted installs that want plain
// `@sam ++` karma add channels:history,groups:history via SLACK_OAUTH_SCOPES.
const DefaultOAuthScopes = "app_mentions:read,chat:write,commands,reactions:read,usergroups:read,users:read"

const oauthStateNonceLen = 16

// oauthStateTimestampLen is the big-endian int64 unix-seconds prefix bound into the
// signed state so a captured state expires (single-use would need shared storage).
const oauthStateTimestampLen = 8

// oauthStateMaxAge bounds how long a minted state stays valid.
const oauthStateMaxAge = 10 * time.Minute

type WorkspaceInstaller interface {
	UpsertInstallation(ctx context.Context, teamID, botToken string) error
}

// OAuthHandler serves /slack/install and /slack/oauth/callback.
type OAuthHandler struct {
	clientID        string
	clientSecret    string
	redirectBaseURL string
	stateSecret     string
	scopes          string
	installer       WorkspaceInstaller
	httpClient      *http.Client
}

// NewOAuthHandler builds the OAuth flow. stateSecret should be a server-only value (e.g. SLACK_SIGNING_SECRET)
// used to HMAC-sign the OAuth state parameter so CSRF does not rely on cookies (fragile on cross-site redirects).
// scopes is the comma-separated bot scope list; empty means DefaultOAuthScopes.
func NewOAuthHandler(clientID, clientSecret, redirectBaseURL string, installer WorkspaceInstaller, stateSecret, scopes string) *OAuthHandler {
	if scopes == "" {
		scopes = DefaultOAuthScopes
	}
	return &OAuthHandler{
		scopes:          scopes,
		clientID:        clientID,
		clientSecret:    clientSecret,
		redirectBaseURL: strings.TrimRight(redirectBaseURL, "/"),
		stateSecret:     stateSecret,
		installer:       installer,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// Scopes is the comma-separated bot scope list /slack/install requests.
func (h *OAuthHandler) Scopes() string { return h.scopes }

func (h *OAuthHandler) redirectURI(r *http.Request) string {
	return h.baseURL(r) + "/slack/oauth/callback"
}

func (h *OAuthHandler) installURL(r *http.Request) string {
	return h.baseURL(r) + "/slack/install"
}

func (h *OAuthHandler) baseURL(r *http.Request) string {
	if h.redirectBaseURL != "" {
		return h.redirectBaseURL
	}
	return inferPublicBaseURL(r)
}

// Install redirects to Slack's OAuth authorize URL (state is HMAC-signed, no cookie).
func (h *OAuthHandler) Install(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	state, err := newSignedOAuthState(h.stateSecret)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Do not send user_scope for bot-only OAuth; an empty user_scope has produced Slack
	// callbacks with state= (empty). Omit the parameter entirely.
	q := url.Values{}
	q.Set("client_id", h.clientID)
	q.Set("scope", h.scopes)
	q.Set("redirect_uri", h.redirectURI(r))
	q.Set("state", state)

	http.Redirect(w, r, "https://slack.com/oauth/v2/authorize?"+q.Encode(), http.StatusFound)
}

// Callback exchanges the code for a bot token and stores it encrypted.
func (h *OAuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if errParam := r.FormValue("error"); errParam != "" {
		http.Error(w, "Slack OAuth error: "+errParam, http.StatusBadRequest)
		return
	}

	state := r.FormValue("state")
	code := r.FormValue("code")
	redirectURI := h.redirectURI(r)

	switch {
	case state == "" && code == "":
		http.Redirect(w, r, h.installURL(r), http.StatusFound)
		return
	case state == "":
		// Installs started on Slack's side (Marketplace button, shareable URL, app settings)
		// carry no state. Exchange the code directly so the user does a single OAuth pass:
		// there is no user session to bind, so the worst a forged callback can do is install
		// the bot into the forger's own workspace. Those flows send no redirect_uri, so the
		// exchange must omit it too.
		redirectURI = ""
	case !verifySignedOAuthState(h.stateSecret, state):
		http.Error(w, "invalid OAuth state", http.StatusBadRequest)
		return
	case code == "":
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	teamID, token, err := h.exchangeCode(r.Context(), code, redirectURI)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	if err := h.installer.UpsertInstallation(r.Context(), teamID, token); err != nil {
		http.Error(w, "failed to save installation", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "<!doctype html><title>Installed</title><p>PlusPlus is installed for this workspace. You can close this tab and return to Slack.</p>")
}

type oauthV2AccessResponse struct {
	OK          bool   `json:"ok"`
	Error       string `json:"error"`
	AccessToken string `json:"access_token"`
	Team        struct {
		ID string `json:"id"`
	} `json:"team"`
}

func (h *OAuthHandler) exchangeCode(ctx context.Context, code, redirectURI string) (teamID, accessToken string, err error) {
	form := url.Values{}
	form.Set("client_id", h.clientID)
	form.Set("client_secret", h.clientSecret)
	form.Set("code", code)
	if redirectURI != "" {
		form.Set("redirect_uri", redirectURI)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://slack.com/api/oauth.v2.access", strings.NewReader(form.Encode()))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("oauth.v2.access request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("read oauth response: %w", err)
	}

	var out oauthV2AccessResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return "", "", fmt.Errorf("decode oauth response: %w", err)
	}

	if !out.OK {
		if out.Error != "" {
			return "", "", fmt.Errorf("oauth.v2.access: %s", out.Error)
		}
		return "", "", fmt.Errorf("oauth.v2.access failed")
	}

	if out.Team.ID == "" || out.AccessToken == "" {
		return "", "", fmt.Errorf("oauth response missing team or token")
	}

	return out.Team.ID, out.AccessToken, nil
}

// newSignedOAuthState returns an opaque state:
// base64url(nonce || ts || HMAC-SHA256(secret, nonce || ts)). The timestamp is
// covered by the MAC so it cannot be rolled forward without the secret.
func newSignedOAuthState(secret string) (string, error) {
	if secret == "" {
		return "", fmt.Errorf("empty state secret")
	}
	payload := make([]byte, oauthStateNonceLen+oauthStateTimestampLen)
	if _, err := rand.Read(payload[:oauthStateNonceLen]); err != nil {
		return "", err
	}
	binary.BigEndian.PutUint64(payload[oauthStateNonceLen:], uint64(time.Now().Unix()))

	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(payload)
	raw := mac.Sum(payload)
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func verifySignedOAuthState(secret string, state string) bool {
	if secret == "" || state == "" {
		return false
	}
	raw, err := base64.RawURLEncoding.DecodeString(state)
	if err != nil {
		return false
	}
	const payloadLen = oauthStateNonceLen + oauthStateTimestampLen
	if len(raw) != payloadLen+sha256.Size {
		return false
	}
	payload := raw[:payloadLen]
	tag := raw[payloadLen:]
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(payload)
	if !hmac.Equal(tag, mac.Sum(nil)) {
		return false
	}

	ts := int64(binary.BigEndian.Uint64(payload[oauthStateNonceLen:]))
	skew := time.Now().Unix() - ts
	if skew < 0 {
		skew = -skew
	}
	return skew <= int64(oauthStateMaxAge.Seconds())
}

func inferPublicBaseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	host := r.Host
	if h := r.Header.Get("X-Forwarded-Host"); h != "" {
		host = h
	}
	return scheme + "://" + host
}
