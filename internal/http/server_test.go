package http

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

type readAllHandler struct{}

func (readAllHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if _, err := io.ReadAll(r.Body); err != nil {
		http.Error(w, "too large", http.StatusRequestEntityTooLarge)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func TestHandlerCapsRequestBody(t *testing.T) {
	h := NewServer(readAllHandler{}, readAllHandler{}, nil, nil, nil).Handler()

	for _, tc := range []struct {
		size int
		want int
	}{{maxBodyBytes, http.StatusOK}, {maxBodyBytes + 1, http.StatusRequestEntityTooLarge}} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/slack/events", bytes.NewReader(make([]byte, tc.size))))
		if rec.Code != tc.want {
			t.Errorf("body %d bytes: got %d, want %d", tc.size, rec.Code, tc.want)
		}
	}
}

func TestHealthz(t *testing.T) {
	rec := httptest.NewRecorder()
	NewServer(readAllHandler{}, readAllHandler{}, nil, nil, nil).Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	var body struct {
		Status string `json:"status"`
	}
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &body) != nil || body.Status != "ok" {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
}

func TestOAuthRoutesWithoutOAuthReturn503(t *testing.T) {
	h := NewServer(readAllHandler{}, readAllHandler{}, nil, nil, nil).Handler()
	for _, path := range []string{"/slack/install", "/slack/oauth/callback"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("%s: status %d, want 503", path, rec.Code)
		}
	}
}
