package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appslack "plusplus/internal/slack"
)

// Every Slack route must reject an unsigned request (AGENTS.md rule 1). NewServer accepts
// any handler, so this checks the wiring main actually uses.
func TestSlackRoutesRejectUnsignedRequests(t *testing.T) {
	h := slackServer("secret",
		appslack.NewEventsProcessor(nil, nil, nil, nil),
		appslack.NewCommandsProcessor(nil, nil, "test"),
		appslack.NewInteractionsProcessor(nil),
		nil, nil,
	).Handler()

	for _, path := range []string{"/slack/events", "/slack/commands", "/slack/interactions"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"type":"url_verification","challenge":"x"}`)))
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("unsigned POST %s: status %d, want 401", path, rec.Code)
		}
	}
}
