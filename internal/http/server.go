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
	if oauthInstall != nil {
		mux.HandleFunc("/slack/install", oauthInstall)
	}
	if oauthCallback != nil {
		mux.HandleFunc("/slack/oauth/callback", oauthCallback)
	}

	return &Server{mux: mux}
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
