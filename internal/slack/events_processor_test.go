package slack

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"plusplus/internal/domain"
)

func TestEventsProcessorURLVerification(t *testing.T) {
	processor := NewEventsProcessor(fakeKarmaActionService{}, nil, nil, &fakeWebClient{})
	payload := []byte(`{"type":"url_verification","challenge":"abc123"}`)
	req := httptest.NewRequest(http.MethodPost, "/slack/events", bytes.NewReader(payload))
	addSlackSignatureHeaders(req, "secret", payload)

	rec := httptest.NewRecorder()
	serveEvent(processor, rec, req)

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
	processor := NewEventsProcessor(fakeKarmaActionService{}, nil, nil, web)
	payload := []byte(`{
		"type":"event_callback",
		"team_id":"T1",
		"event":{"type":"app_mention","user":"U1","text":"<@UBOT> <@U2> ++++","channel":"C1","event_ts":"123.4"}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/slack/events", bytes.NewReader(payload))
	addSlackSignatureHeaders(req, "secret", payload)

	rec := httptest.NewRecorder()
	serveEvent(processor, rec, req)

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
	processor := NewEventsProcessor(fakeKarmaActionService{}, nil, nil, web)
	payload := []byte(`{
		"type":"event_callback",
		"team_id":"T1",
		"event":{"type":"app_mention","user":"U1","text":"<@UBOT> Bravo <@U2> ++++++ , from <@U3> +++++","channel":"C1","event_ts":"123.4"}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/slack/events", bytes.NewReader(payload))
	addSlackSignatureHeaders(req, "secret", payload)

	rec := httptest.NewRecorder()
	serveEvent(processor, rec, req)

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

func TestEventsProcessorSkipsMessageWhenSameAsAppMention(t *testing.T) {
	web := &fakeWebClient{}
	processor := NewEventsProcessor(fakeKarmaActionService{}, nil, nil, web)
	payload := []byte(`{
		"type":"event_callback",
		"team_id":"T1",
		"authorizations":[{"user_id":"UBOT","is_bot":true}],
		"event":{"type":"message","user":"U1","text":"<@UBOT> <@U2> ++++","channel":"C1","event_ts":"123.4"}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/slack/events", bytes.NewReader(payload))
	addSlackSignatureHeaders(req, "secret", payload)

	rec := httptest.NewRecorder()
	serveEvent(processor, rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	if len(web.posts) != 0 {
		t.Fatalf("expected duplicate message event skipped, got %d posts", len(web.posts))
	}
}

func TestEventsProcessorSkipsMessageChanged(t *testing.T) {
	web := &fakeWebClient{}
	processor := NewEventsProcessor(fakeKarmaActionService{}, nil, nil, web)
	payload := []byte(`{
		"type":"event_callback",
		"team_id":"T1",
		"event":{"type":"message","subtype":"message_changed","user":"U1","text":"<@U2> ++++","channel":"C1","event_ts":"123.4"}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/slack/events", bytes.NewReader(payload))
	addSlackSignatureHeaders(req, "secret", payload)

	rec := httptest.NewRecorder()
	serveEvent(processor, rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	if len(web.posts) != 0 {
		t.Fatalf("expected message_changed skipped, got %d posts", len(web.posts))
	}
}

func TestEventsProcessorAmbientMessagePostsMessage(t *testing.T) {
	web := &fakeWebClient{}
	processor := NewEventsProcessor(fakeKarmaActionService{}, nil, nil, web)
	payload := []byte(`{
		"type":"event_callback",
		"team_id":"T1",
		"event":{"type":"message","user":"U1","text":"<@U2> ++++","channel":"C1","event_ts":"123.4"}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/slack/events", bytes.NewReader(payload))
	addSlackSignatureHeaders(req, "secret", payload)

	rec := httptest.NewRecorder()
	serveEvent(processor, rec, req)

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
	processor := NewEventsProcessor(fakeKarmaActionService{}, settings, nil, web)
	payload := []byte(`{
		"type":"event_callback",
		"team_id":"T1",
		"event":{"type":"app_mention","user":"U1","text":"<@UBOT> <@U2> ++++","channel":"C1","event_ts":"123.4"}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/slack/events", bytes.NewReader(payload))
	addSlackSignatureHeaders(req, "secret", payload)

	rec := httptest.NewRecorder()
	serveEvent(processor, rec, req)

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
	processor := NewEventsProcessor(fakeKarmaActionService{}, nil, nil, &fakeWebClient{})
	payload := []byte(`{"type":"url_verification","challenge":"abc123"}`)
	req := httptest.NewRequest(http.MethodPost, "/slack/events", bytes.NewReader(payload))
	req.Header.Set("X-Slack-Request-Timestamp", fmt.Sprintf("%d", time.Now().Unix()))
	req.Header.Set("X-Slack-Signature", "v0=invalid")

	rec := httptest.NewRecorder()
	serveEvent(processor, rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
}

func TestEventsProcessorSubteamKarmaCombinesLines(t *testing.T) {
	web := &fakeWebClient{}
	lister := &fakeUserGroupLister{ids: []string{"U2", "U3"}}
	processor := NewEventsProcessor(subteamKarmaFake{}, nil, lister, web)
	payload := []byte(`{
		"type":"event_callback",
		"team_id":"T1",
		"event":{"type":"app_mention","user":"U1","text":"<@UBOT> <!subteam^S12345|@admins> ++++","channel":"C1","event_ts":"123.4"}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/slack/events", bytes.NewReader(payload))
	addSlackSignatureHeaders(req, "secret", payload)

	rec := httptest.NewRecorder()
	serveEvent(processor, rec, req)

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
	processor := NewEventsProcessor(fakeKarmaActionService{}, nil, nil, web)
	payload := []byte(`{
		"type":"event_callback",
		"team_id":"T1",
		"event":{"type":"app_mention","user":"U1","text":"<@UBOT> <!subteam^S1|@admins> ++","channel":"C1","event_ts":"123.4"}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/slack/events", bytes.NewReader(payload))
	addSlackSignatureHeaders(req, "secret", payload)

	rec := httptest.NewRecorder()
	serveEvent(processor, rec, req)

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
	processor := NewEventsProcessor(fakeKarmaActionService{}, nil, lister, web)
	payload := []byte(`{
		"type":"event_callback",
		"team_id":"T1",
		"event":{"type":"app_mention","user":"U1","text":"<@UBOT> <!subteam^S1|@admins> ++","channel":"C1","event_ts":"123.4"}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/slack/events", bytes.NewReader(payload))
	addSlackSignatureHeaders(req, "secret", payload)

	rec := httptest.NewRecorder()
	serveEvent(processor, rec, req)

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

type recordingKarmaService struct{ targets *[]string }

func (f recordingKarmaService) HandleAction(_ context.Context, action domain.KarmaAction) (domain.KarmaResult, error) {
	*f.targets = append(*f.targets, action.TargetUserID)
	return domain.KarmaResult{Message: "ok"}, nil
}

func TestEventsProcessorNeverCreditsTheBot(t *testing.T) {
	cases := map[string][]string{
		"<@U2> <@UBOT> ++": {"U2"}, // bot mention between target and run
		"<@UBOT> ++":       nil,    // "thanks @plusplus ++"
	}
	for text, want := range cases {
		var got []string
		processor := NewEventsProcessor(recordingKarmaService{targets: &got}, nil, nil, &fakeWebClient{})
		payload := []byte(`{
			"type":"event_callback",
			"team_id":"T1",
			"authorizations":[{"user_id":"UBOT","is_bot":true}],
			"event":{"type":"app_mention","user":"U1","text":"` + text + `","channel":"C1","event_ts":"123.4"}
		}`)
		req := httptest.NewRequest(http.MethodPost, "/slack/events", bytes.NewReader(payload))
		addSlackSignatureHeaders(req, "secret", payload)
		serveEvent(processor, httptest.NewRecorder(), req)

		if len(got) != len(want) || (len(want) > 0 && got[0] != want[0]) {
			t.Fatalf("%q: expected targets %v, got %v", text, want, got)
		}
	}
}

func groupMembers(n int) []string {
	ids := make([]string, n)
	for i := range ids {
		ids[i] = fmt.Sprintf("U%03d", i)
	}
	return ids
}

func TestEventsProcessorGroupKarmaSizeLimit(t *testing.T) {
	cases := map[string]struct {
		members   []string
		wantCalls int
		wantText  []string
	}{
		"at the limit":            {members: groupMembers(50), wantCalls: 50},
		"one over the limit":      {members: groupMembers(51), wantCalls: 0, wantText: []string{"51", "50"}},
		"duplicates counted once": {members: append(groupMembers(40), groupMembers(20)...), wantCalls: 40},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var calls []string
			web := &fakeWebClient{}
			p := NewEventsProcessor(countingKarma{calls: &calls}, nil, &fakeUserGroupLister{ids: tc.members}, web)
			if got := deliver(context.Background(), p, "", "<!subteam^S1|@team> ++"); got != http.StatusOK {
				t.Fatalf("status %d", got)
			}
			if len(calls) != tc.wantCalls {
				t.Fatalf("karma applied %d times, want %d", len(calls), tc.wantCalls)
			}
			if tc.wantText != nil {
				if len(web.posts) != 1 {
					t.Fatalf("posts %+v, want one message", web.posts)
				}
				for _, want := range tc.wantText {
					if !strings.Contains(web.posts[0].text, want) {
						t.Errorf("message %q does not mention %s", web.posts[0].text, want)
					}
				}
			}
		})
	}
}

// A too-large group is skipped, not the whole message: individual users still get karma.
func TestEventsProcessorTooLargeGroupStillCreditsIndividuals(t *testing.T) {
	var calls []string
	web := &fakeWebClient{}
	p := NewEventsProcessor(countingKarma{calls: &calls}, nil, &fakeUserGroupLister{ids: groupMembers(51)}, web)
	if got := deliver(context.Background(), p, "", "<!subteam^S1|@team> ++ <@U9> ++"); got != http.StatusOK {
		t.Fatalf("status %d", got)
	}
	if strings.Join(calls, ",") != "U9" {
		t.Fatalf("karma calls %v, want only U9", calls)
	}
	if len(web.posts) != 2 || web.posts[0].threadTS != "5.1" {
		t.Fatalf("posts %+v; want the too-large notice in the thread, then U9's reply", web.posts)
	}
}

// Pins today's status for a karma failure inside a user-group award: 502 with the error as
// the body (single-user failures are 500). Item 13's refactor must keep it.
func TestEventsProcessorGroupKarmaFailureIs502(t *testing.T) {
	var calls []string
	p := NewEventsProcessor(countingKarma{calls: &calls, fail: map[string]bool{"U001": true}}, nil, &fakeUserGroupLister{ids: groupMembers(3)}, &fakeWebClient{})
	if got := deliver(context.Background(), p, "", "<!subteam^S1|@team> ++"); got != http.StatusBadGateway {
		t.Fatalf("status %d, want 502", got)
	}
}

func karmaEnvelope(text string) EventEnvelope {
	return EventEnvelope{
		Type:           "event_callback",
		TeamID:         "T1",
		Authorizations: []SlackAuthorization{{UserID: "UBOT", IsBot: true}},
		Event:          SlackEvent{Type: "app_mention", User: "U2", Text: "<@UBOT> " + text, Channel: "C1", TS: "5.1", EventTS: "5.1"},
	}
}

func TestHandleKarmaMessageAppliesAndReplies(t *testing.T) {
	var calls []string
	web := &fakeWebClient{}
	p := NewEventsProcessor(countingKarma{calls: &calls}, nil, nil, web)
	if err := p.handleKarmaMessage(context.Background(), karmaEnvelope("<@U3> ++")); err != nil {
		t.Fatal(err)
	}
	if strings.Join(calls, ",") != "U3" || len(web.posts) != 1 || web.posts[0].threadTS != "5.1" {
		t.Fatalf("calls %v, posts %+v", calls, web.posts)
	}
}

func TestHandleKarmaMessageKarmaFailureMapsTo500(t *testing.T) {
	var calls []string
	p := NewEventsProcessor(countingKarma{calls: &calls, fail: map[string]bool{"U3": true}}, nil, nil, &fakeWebClient{})
	err := p.handleKarmaMessage(context.Background(), karmaEnvelope("<@U3> ++"))
	var failure *eventFailure
	if !errors.As(err, &failure) || failure.status != http.StatusInternalServerError || failure.body != "failed to apply karma" {
		t.Fatalf("err = %v; want an eventFailure with status 500", err)
	}
}
