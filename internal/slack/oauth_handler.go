package slack

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	oauthStateCookie = "slack_oauth_state"
	oauthScopes      = "app_mentions:read,channels:history,chat:write,commands,usergroups:read"
)

type WorkspaceInstaller interface {
	UpsertInstallation(ctx context.Context, teamID, botToken string) error
}

// OAuthHandler serves /slack/install and /slack/oauth/callback.
type OAuthHandler struct {
	clientID        string
	clientSecret    string
	redirectBaseURL string
	installer       WorkspaceInstaller
	httpClient      *http.Client
}

func NewOAuthHandler(clientID, clientSecret, redirectBaseURL string, installer WorkspaceInstaller) *OAuthHandler {
	return &OAuthHandler{
		clientID:        clientID,
		clientSecret:    clientSecret,
		redirectBaseURL: strings.TrimRight(redirectBaseURL, "/"),
		installer:       installer,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (h *OAuthHandler) redirectURI(r *http.Request) string {
	base := h.redirectBaseURL
	if base == "" {
		base = inferPublicBaseURL(r)
	}
	return base + "/slack/oauth/callback"
}

// Install redirects to Slack's OAuth authorize URL and sets a CSRF cookie.
func (h *OAuthHandler) Install(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	stateBytes := make([]byte, 24)
	if _, err := rand.Read(stateBytes); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	state := base64.RawURLEncoding.EncodeToString(stateBytes)

	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookie,
		Value:    state,
		Path:     "/",
		MaxAge:   600,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
	})

	q := url.Values{}
	q.Set("client_id", h.clientID)
	q.Set("user_scope", "")
	q.Set("scope", oauthScopes)
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
	cookie, err := r.Cookie(oauthStateCookie)
	if err != nil || cookie.Value == "" || cookie.Value != state {
		http.Error(w, "invalid OAuth state", http.StatusBadRequest)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
	})

	code := r.FormValue("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	teamID, token, err := h.exchangeCode(r.Context(), code, h.redirectURI(r))
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
	form.Set("redirect_uri", redirectURI)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://slack.com/api/oauth.v2.access", strings.NewReader(form.Encode()))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("oauth.v2.access request: %w", err)
	}
	defer resp.Body.Close()

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
