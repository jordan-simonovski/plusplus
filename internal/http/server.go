package http

import (
	"encoding/json"
	"net/http"
	"time"
)

type EventsHandler interface {
	HandleEvents(http.ResponseWriter, *http.Request)
}

type CommandsHandler interface {
	HandleCommand(http.ResponseWriter, *http.Request)
}

type InteractionsHandler interface {
	HandleInteraction(http.ResponseWriter, *http.Request)
}

type Server struct {
	mux *http.ServeMux
}

func NewServer(eventsHandler EventsHandler, commandsHandler CommandsHandler, interactionsHandler InteractionsHandler, oauthInstall, oauthCallback http.HandlerFunc) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", handleHealth)
	mux.HandleFunc("/slack/events", eventsHandler.HandleEvents)
	mux.HandleFunc("/slack/commands", commandsHandler.HandleCommand)
	if interactionsHandler != nil {
		mux.HandleFunc("/slack/interactions", interactionsHandler.HandleInteraction)
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

func (s *Server) Handler() http.Handler {
	return s.mux
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
