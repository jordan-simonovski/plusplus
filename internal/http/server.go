package http

import (
	"encoding/json"
	"net/http"
	"time"
)

type Server struct {
	mux *http.ServeMux
}

// NewServer routes the Slack endpoints. The events, commands and interactions handlers
// must already be wrapped in slack.VerifiedBody; cmd/server's slackServer does that, and
// its test checks every Slack route rejects unsigned requests.
func NewServer(events, commands, interactions http.Handler, oauthInstall, oauthCallback http.HandlerFunc) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", handleHealth)
	mux.Handle("POST /slack/events", events)
	mux.Handle("POST /slack/commands", commands)
	if interactions != nil {
		mux.Handle("POST /slack/interactions", interactions)
	}
	if oauthInstall == nil {
		oauthInstall = oauthInstallDisabled
	}
	if oauthCallback == nil {
		oauthCallback = oauthCallbackDisabled
	}
	mux.HandleFunc("/slack/install", oauthInstall)
	mux.HandleFunc("/slack/oauth/callback", oauthCallback)

	return &Server{mux: mux}
}

func oauthInstallDisabled(w http.ResponseWriter, _ *http.Request) {
	writeOAuthDisabled(w)
}

func oauthCallbackDisabled(w http.ResponseWriter, _ *http.Request) {
	writeOAuthDisabled(w)
}

func writeOAuthDisabled(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = w.Write([]byte("OAuth is not enabled. Set SLACK_CLIENT_ID, SLACK_CLIENT_SECRET, and TOKEN_ENCRYPTION_KEY on the server, then redeploy.\n"))
}

// maxBodyBytes caps every request body. Slack payloads are a few KB; handlers
// read the whole body before verifying the signature, so this bounds what an
// unauthenticated client can make the process buffer.
const maxBodyBytes = 1 << 20

func (s *Server) Handler() http.Handler {
	return http.MaxBytesHandler(s.mux, maxBodyBytes)
}

type healthResponse struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(healthResponse{
		Status:    "ok",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}
