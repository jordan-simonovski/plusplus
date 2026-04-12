package slack

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"plusplus/internal/domain"
)

func TestEventsProcessorURLVerification(t *testing.T) {
	processor := NewEventsProcessor("secret", fakeKarmaActionService{}, nil, &fakeWebClient{})
	payload := []byte(`{"type":"url_verification","challenge":"abc123"}`)
	req := httptest.NewRequest(http.MethodPost, "/slack/events", bytes.NewReader(payload))
	addSlackSignatureHeaders(req, "secret", payload)

	rec := httptest.NewRecorder()
	processor.ProcessEvent(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body["challenge"] != "abc123" {
		t.Fatalf("unexpected challenge response: %+v", body)
	}
}

func TestEventsProcessorAppMentionPostsMessage(t *testing.T) {
	web := &fakeWebClient{}
	processor := NewEventsProcessor("secret", fakeKarmaActionService{}, nil, web)
	payload := []byte(`{
		"type":"event_callback",
		"team_id":"T1",
		"event":{"type":"app_mention","user":"U1","text":"<@UBOT> <@U2> ++++","channel":"C1","event_ts":"123.4"}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/slack/events", bytes.NewReader(payload))
	addSlackSignatureHeaders(req, "secret", payload)

	rec := httptest.NewRecorder()
	processor.ProcessEvent(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	if web.channelID != "C1" {
		t.Fatalf("unexpected channel: %s", web.channelID)
	}
	if web.threadTS != "123.4" {
		t.Fatalf("expected thread reply, got %q", web.threadTS)
	}
}

func TestEventsProcessorAmbientMessagePostsMessage(t *testing.T) {
	web := &fakeWebClient{}
	processor := NewEventsProcessor("secret", fakeKarmaActionService{}, nil, web)
	payload := []byte(`{
		"type":"event_callback",
		"team_id":"T1",
		"event":{"type":"message","user":"U1","text":"<@U2> ++++","channel":"C1","event_ts":"123.4"}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/slack/events", bytes.NewReader(payload))
	addSlackSignatureHeaders(req, "secret", payload)

	rec := httptest.NewRecorder()
	processor.ProcessEvent(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	if web.channelID != "C1" {
		t.Fatalf("unexpected channel: %s", web.channelID)
	}
	if web.threadTS != "123.4" {
		t.Fatalf("expected thread reply, got %q", web.threadTS)
	}
}

func TestEventsProcessorUsesChannelModeWhenConfigured(t *testing.T) {
	web := &fakeWebClient{}
	settings := &fakeReplyModeService{mode: ReplyModeChannel}
	processor := NewEventsProcessor("secret", fakeKarmaActionService{}, settings, web)
	payload := []byte(`{
		"type":"event_callback",
		"team_id":"T1",
		"event":{"type":"app_mention","user":"U1","text":"<@UBOT> <@U2> ++++","channel":"C1","event_ts":"123.4"}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/slack/events", bytes.NewReader(payload))
	addSlackSignatureHeaders(req, "secret", payload)

	rec := httptest.NewRecorder()
	processor.ProcessEvent(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	if web.threadTS != "" {
		t.Fatalf("expected in-channel reply with empty thread ts, got %q", web.threadTS)
	}
}

func TestEventsProcessorRejectsInvalidSignature(t *testing.T) {
	processor := NewEventsProcessor("secret", fakeKarmaActionService{}, nil, &fakeWebClient{})
	payload := []byte(`{"type":"url_verification","challenge":"abc123"}`)
	req := httptest.NewRequest(http.MethodPost, "/slack/events", bytes.NewReader(payload))
	req.Header.Set("X-Slack-Request-Timestamp", fmt.Sprintf("%d", time.Now().Unix()))
	req.Header.Set("X-Slack-Signature", "v0=invalid")

	rec := httptest.NewRecorder()
	processor.ProcessEvent(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
}

type fakeKarmaActionService struct{}

func (f fakeKarmaActionService) HandleAction(_ context.Context, action domain.KarmaAction) (domain.KarmaResult, error) {
	return domain.KarmaResult{
		ShouldPersist: true,
		Message:       "applied " + action.SymbolRun,
	}, nil
}

type fakeWebClient struct {
	channelID string
	text      string
	threadTS  string
}

type fakeReplyModeService struct {
	mode ReplyMode
}

func (f *fakeReplyModeService) GetReplyMode(_ context.Context, _ string, _ string) (ReplyMode, error) {
	return f.mode, nil
}

func (f *fakeWebClient) PostMessage(_ context.Context, channelID, text, threadTS string) error {
	f.channelID = channelID
	f.text = text
	f.threadTS = threadTS
	return nil
}

func addSlackSignatureHeaders(req *http.Request, secret string, body []byte) {
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	req.Header.Set("X-Slack-Request-Timestamp", timestamp)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("v0:" + timestamp + ":"))
	mac.Write(body)
	req.Header.Set("X-Slack-Signature", "v0="+hex.EncodeToString(mac.Sum(nil)))
}
