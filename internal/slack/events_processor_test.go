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
	processor := NewEventsProcessor("secret", fakeKarmaActionService{}, nil, nil, &fakeWebClient{})
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
	processor := NewEventsProcessor("secret", fakeKarmaActionService{}, nil, nil, web)
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
	if len(web.posts) != 1 {
		t.Fatalf("expected 1 post, got %d", len(web.posts))
	}
	if web.posts[0].channelID != "C1" {
		t.Fatalf("unexpected channel: %s", web.posts[0].channelID)
	}
	if web.posts[0].threadTS != "123.4" {
		t.Fatalf("expected thread reply, got %q", web.posts[0].threadTS)
	}
}

func TestEventsProcessorMultipleKarmaTargetsPostSeparateMessages(t *testing.T) {
	web := &fakeWebClient{}
	processor := NewEventsProcessor("secret", fakeKarmaActionService{}, nil, nil, web)
	payload := []byte(`{
		"type":"event_callback",
		"team_id":"T1",
		"event":{"type":"app_mention","user":"U1","text":"<@UBOT> Bravo <@U2> ++++++ , from <@U3> +++++","channel":"C1","event_ts":"123.4"}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/slack/events", bytes.NewReader(payload))
	addSlackSignatureHeaders(req, "secret", payload)

	rec := httptest.NewRecorder()
	processor.ProcessEvent(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	if len(web.posts) != 2 {
		t.Fatalf("expected 2 posts, got %d", len(web.posts))
	}
	for i, want := range []string{"applied ++++++", "applied +++++"} {
		if web.posts[i].text != want {
			t.Fatalf("post %d: got %q want %q", i, web.posts[i].text, want)
		}
	}
}

func TestEventsProcessorAmbientMessagePostsMessage(t *testing.T) {
	web := &fakeWebClient{}
	processor := NewEventsProcessor("secret", fakeKarmaActionService{}, nil, nil, web)
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
	if len(web.posts) != 1 {
		t.Fatalf("expected 1 post, got %d", len(web.posts))
	}
	if web.posts[0].channelID != "C1" {
		t.Fatalf("unexpected channel: %s", web.posts[0].channelID)
	}
	if web.posts[0].threadTS != "123.4" {
		t.Fatalf("expected thread reply, got %q", web.posts[0].threadTS)
	}
}

func TestEventsProcessorUsesChannelModeWhenConfigured(t *testing.T) {
	web := &fakeWebClient{}
	settings := &fakeReplyModeService{mode: ReplyModeChannel}
	processor := NewEventsProcessor("secret", fakeKarmaActionService{}, settings, nil, web)
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
	if len(web.posts) != 1 {
		t.Fatalf("expected 1 post, got %d", len(web.posts))
	}
	if web.posts[0].threadTS != "" {
		t.Fatalf("expected in-channel reply with empty thread ts, got %q", web.posts[0].threadTS)
	}
}

func TestEventsProcessorRejectsInvalidSignature(t *testing.T) {
	processor := NewEventsProcessor("secret", fakeKarmaActionService{}, nil, nil, &fakeWebClient{})
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

func TestEventsProcessorSubteamKarmaCombinesLines(t *testing.T) {
	web := &fakeWebClient{}
	lister := &fakeUserGroupLister{ids: []string{"U2", "U3"}}
	processor := NewEventsProcessor("secret", subteamKarmaFake{}, nil, lister, web)
	payload := []byte(`{
		"type":"event_callback",
		"team_id":"T1",
		"event":{"type":"app_mention","user":"U1","text":"<@UBOT> <!subteam^S12345|@admins> ++++","channel":"C1","event_ts":"123.4"}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/slack/events", bytes.NewReader(payload))
	addSlackSignatureHeaders(req, "secret", payload)

	rec := httptest.NewRecorder()
	processor.ProcessEvent(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	if len(web.posts) != 1 {
		t.Fatalf("expected 1 post, got %d", len(web.posts))
	}
	want := "U2:group\nU3:group"
	if web.posts[0].text != want {
		t.Fatalf("unexpected combined text: %q", web.posts[0].text)
	}
}

func TestEventsProcessorSubteamWithoutListerPostsNotice(t *testing.T) {
	web := &fakeWebClient{}
	processor := NewEventsProcessor("secret", fakeKarmaActionService{}, nil, nil, web)
	payload := []byte(`{
		"type":"event_callback",
		"team_id":"T1",
		"event":{"type":"app_mention","user":"U1","text":"<@UBOT> <!subteam^S1|@admins> ++","channel":"C1","event_ts":"123.4"}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/slack/events", bytes.NewReader(payload))
	addSlackSignatureHeaders(req, "secret", payload)

	rec := httptest.NewRecorder()
	processor.ProcessEvent(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	if len(web.posts) != 1 || web.posts[0].text != "Could not resolve user groups (not configured)." {
		t.Fatalf("unexpected posts: %+v", web.posts)
	}
}

func TestEventsProcessorSubteamEmptyGroupPostsNotice(t *testing.T) {
	web := &fakeWebClient{}
	lister := &fakeUserGroupLister{ids: nil}
	processor := NewEventsProcessor("secret", fakeKarmaActionService{}, nil, lister, web)
	payload := []byte(`{
		"type":"event_callback",
		"team_id":"T1",
		"event":{"type":"app_mention","user":"U1","text":"<@UBOT> <!subteam^S1|@admins> ++","channel":"C1","event_ts":"123.4"}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/slack/events", bytes.NewReader(payload))
	addSlackSignatureHeaders(req, "secret", payload)

	rec := httptest.NewRecorder()
	processor.ProcessEvent(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	if len(web.posts) != 1 || web.posts[0].text != "That user group has no members." {
		t.Fatalf("unexpected posts: %+v", web.posts)
	}
}

type fakeKarmaActionService struct{}

func (f fakeKarmaActionService) HandleAction(_ context.Context, action domain.KarmaAction) (domain.KarmaResult, error) {
	return domain.KarmaResult{
		ShouldPersist: true,
		Message:       "applied " + action.SymbolRun,
	}, nil
}

type fakeUserGroupLister struct {
	ids []string
	err error
}

func (f *fakeUserGroupLister) ListUserGroupMembers(_ context.Context, _, _ string) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.ids, nil
}

type subteamKarmaFake struct{}

func (subteamKarmaFake) HandleAction(_ context.Context, action domain.KarmaAction) (domain.KarmaResult, error) {
	suffix := "single"
	if action.GroupBroadcast {
		suffix = "group"
	}
	return domain.KarmaResult{
		ShouldPersist: true,
		Message:       action.TargetUserID + ":" + suffix,
	}, nil
}

type fakeWebClient struct {
	posts []postedMessage
}

type postedMessage struct {
	channelID string
	text      string
	threadTS  string
}

type fakeReplyModeService struct {
	mode ReplyMode
}

func (f *fakeReplyModeService) GetChannelSettings(_ context.Context, _ string, _ string) (ReplyMode, int, error) {
	return f.mode, domain.DefaultSnarkLevel, nil
}

func (f *fakeWebClient) PostMessage(_ context.Context, _ string, channelID, text, threadTS string) error {
	f.posts = append(f.posts, postedMessage{channelID: channelID, text: text, threadTS: threadTS})
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
