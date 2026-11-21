package slack

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func interactionBody(payload string) []byte {
	if payload == "" {
		return []byte("foo=bar")
	}
	return []byte("payload=" + url.QueryEscape(payload))
}

func sendInteraction(p *InteractionsProcessor, body []byte, sign bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/slack/interactions", bytes.NewReader(body))
	if sign {
		addSlackSignatureHeaders(req, "secret", body)
	}
	rec := httptest.NewRecorder()
	VerifiedBody("secret", p.HandleInteraction).ServeHTTP(rec, req)
	return rec
}

func panelAction(actionID, value, responseURL string) string {
	return `{"type":"block_actions","user":{"id":"U1"},"team":{"id":"T1"},"channel":{"id":"C1"},
		"response_url":"` + responseURL + `",
		"actions":[{"action_id":"` + actionID + `","selected_option":{"value":"` + value + `"}}]}`
}

// panelPost is what the processor sent to response_url.
type panelPost struct {
	url  string
	text string
	body string
}

// interactionsUnderTest returns a processor whose response_url posts are recorded.
func interactionsUnderTest(settings *recordingSettings, reactions *recordingReactions, status int) (*InteractionsProcessor, *[]panelPost) {
	var posts []panelPost
	p := NewInteractionsProcessor(settings, "1.2.3").WithReactionSettings(reactions)
	p.httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		raw, _ := io.ReadAll(r.Body)
		var msg struct {
			Text            string `json:"text"`
			ReplaceOriginal bool   `json:"replace_original"`
		}
		_ = json.Unmarshal(raw, &msg)
		if !msg.ReplaceOriginal {
			return nil, errors.New("panel must replace the original")
		}
		posts = append(posts, panelPost{url: r.URL.String(), text: msg.Text, body: string(raw)})
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader("ok"))}, nil
	})}
	return p, &posts
}

const hook = "https://hooks.slack.com/actions/T1/1/abc"

func TestInteractionsSettingsPanel(t *testing.T) {
	cases := map[string]struct {
		body       []byte
		sign       bool
		storeErr   error
		wantCode   int
		wantModes  string
		wantLevels string
		wantNotice string // the refreshed panel's notice; empty means no refresh
	}{
		"unsigned":             {body: interactionBody(panelAction(actionSnarkLevel, "7", hook)), wantCode: http.StatusUnauthorized},
		"no payload field":     {body: interactionBody(""), sign: true, wantCode: http.StatusBadRequest},
		"not block_actions":    {body: interactionBody(`{"type":"view_submission"}`), sign: true, wantCode: http.StatusOK},
		"unknown action":       {body: interactionBody(panelAction("other", "7", hook)), sign: true, wantCode: http.StatusOK},
		"snark level":          {body: interactionBody(panelAction(actionSnarkLevel, "7", hook)), sign: true, wantCode: http.StatusOK, wantLevels: "[7]", wantNotice: "Snark level set to 7."},
		"snark 10, the max":    {body: interactionBody(panelAction(actionSnarkLevel, "10", hook)), sign: true, wantCode: http.StatusOK, wantLevels: "[10]", wantNotice: "Snark level set to 10."},
		"snark out of range":   {body: interactionBody(panelAction(actionSnarkLevel, "11", hook)), sign: true, wantCode: http.StatusOK, wantLevels: "[]", wantNotice: "That isn't a snark level."},
		"snark not a number":   {body: interactionBody(panelAction(actionSnarkLevel, "abc", hook)), sign: true, wantCode: http.StatusOK, wantLevels: "[]", wantNotice: "That isn't a snark level."},
		"reply mode channel":   {body: interactionBody(panelAction(actionReplyMode, "channel", hook)), sign: true, wantCode: http.StatusOK, wantModes: "[channel]", wantNotice: "Reply mode set to channel."},
		"reply mode unknown":   {body: interactionBody(panelAction(actionReplyMode, "dm", hook)), sign: true, wantCode: http.StatusOK, wantModes: "[]", wantNotice: "That isn't a reply mode."},
		"store error is a 500": {body: interactionBody(panelAction(actionSnarkLevel, "7", hook)), sign: true, storeErr: errors.New("dynamo down"), wantCode: http.StatusInternalServerError, wantLevels: "[7]"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			settings := &recordingSettings{err: tc.storeErr}
			p, posts := interactionsUnderTest(settings, &recordingReactions{}, http.StatusOK)
			rec := sendInteraction(p, tc.body, tc.sign)
			if rec.Code != tc.wantCode {
				t.Fatalf("status %d, want %d (%s)", rec.Code, tc.wantCode, rec.Body.String())
			}
			if tc.wantModes != "" && fmt.Sprint(settings.modes) != tc.wantModes {
				t.Fatalf("modes %v, want %s", settings.modes, tc.wantModes)
			}
			if tc.wantLevels != "" && fmt.Sprint(settings.levels) != tc.wantLevels {
				t.Fatalf("levels %v, want %s", settings.levels, tc.wantLevels)
			}
			if tc.wantNotice == "" {
				if len(*posts) != 0 {
					t.Fatalf("expected no panel refresh, got %+v", *posts)
				}
				return
			}
			if len(*posts) != 1 || (*posts)[0].url != hook || (*posts)[0].text != tc.wantNotice {
				t.Fatalf("panel refresh %+v, want one post to %s with %q", *posts, hook, tc.wantNotice)
			}
		})
	}
}

func TestInteractionsRefreshedPanelShowsNewSettings(t *testing.T) {
	settings := &recordingSettings{}
	p, posts := interactionsUnderTest(settings, &recordingReactions{}, http.StatusOK)
	sendInteraction(p, interactionBody(panelAction(actionReplyMode, "channel", hook)), true)
	sendInteraction(p, interactionBody(panelAction(actionSnarkLevel, "9", hook)), true)

	last := (*posts)[1].body
	for _, want := range []string{
		`"initial_option":{"text":{"text":"In the channel","type":"plain_text"},"value":"channel"}`,
		`"initial_option":{"text":{"text":"9","type":"plain_text"},"value":"9"}`,
		`Snark level set to 9.  ·  PlusPlus v1.2.3`,
		`karma reactions`,
	} {
		if !strings.Contains(last, want) {
			t.Fatalf("refreshed panel missing %s:\n%s", want, last)
		}
	}
}

func TestInteractionsOnlyPostToSlack(t *testing.T) {
	for _, bad := range []string{"https://evil.example/hook", "http://hooks.slack.com/actions/x", "", "https://hooks.slack.com.evil.example/x"} {
		settings := &recordingSettings{}
		p, posts := interactionsUnderTest(settings, &recordingReactions{}, http.StatusOK)
		rec := sendInteraction(p, interactionBody(panelAction(actionSnarkLevel, "7", bad)), true)
		if rec.Code != http.StatusOK || len(settings.levels) != 1 {
			t.Fatalf("%q: the setting should still save (status %d, levels %v)", bad, rec.Code, settings.levels)
		}
		if len(*posts) != 0 {
			t.Fatalf("%q: posted the panel to a non-Slack URL", bad)
		}
	}
}

func TestInteractionsRefreshFailureStillAcks(t *testing.T) {
	settings := &recordingSettings{}
	p, _ := interactionsUnderTest(settings, &recordingReactions{}, http.StatusInternalServerError)
	if rec := sendInteraction(p, interactionBody(panelAction(actionSnarkLevel, "7", hook)), true); rec.Code != http.StatusOK {
		t.Fatalf("a saved setting with a failed redraw should still ack, got %d", rec.Code)
	}
}

// emojiAction is the panel's emoji input (or reset button) firing, with the tier picker
// and workspace checkbox in the message state.
func emojiAction(actionID, typed, tier string, workspace bool) string {
	state := `{}`
	if tier != "" {
		state = `{"emoji_tier_select":{"type":"static_select","selected_option":{"value":"` + tier + `"}}}`
	}
	scope := `[]`
	if workspace {
		scope = `[{"value":"workspace"}]`
	}
	return `{"type":"block_actions","user":{"id":"U1"},"team":{"id":"T1"},"channel":{"id":"C1"},
		"response_url":"` + hook + `",
		"actions":[{"action_id":"` + actionID + `","value":"` + typed + `"}],
		"state":{"values":{"b1":` + state + `,"b2":{"emoji_scope_check":{"type":"checkboxes","selected_options":` + scope + `}},
			"b3":{"emoji_tier_input":{"type":"plain_text_input","value":"` + typed + `"}}}}}`
}

func TestInteractionsChangeKarmaReactions(t *testing.T) {
	cases := map[string]struct {
		payload    string
		err        error
		wantCode   int
		wantCalls  string
		wantNotice string
	}{
		"set a tier for the channel": {payload: emojiAction(actionEmojiInput, ":tada: :rocket:", "5", false), wantCode: http.StatusOK,
			wantCalls: "set workspace=false tier=5 :tada: :rocket:,describe", wantNotice: "tier set"},
		"set a tier for the workspace": {payload: emojiAction(actionEmojiInput, ":tada:", "-2", true), wantCode: http.StatusOK,
			wantCalls: "set workspace=true tier=-2 :tada:,describe", wantNotice: "tier set"},
		"no tier picked": {payload: emojiAction(actionEmojiInput, ":tada:", "", false), wantCode: http.StatusOK,
			wantCalls: "describe", wantNotice: "Pick a tier first, then type the emojis."},
		"a bad input is shown on the panel": {payload: emojiAction(actionEmojiInput, "tada", "5", true), err: &InputError{Msg: "only workspace admins can do that"}, wantCode: http.StatusOK,
			wantCalls: "set workspace=true tier=5 tada,describe", wantNotice: "Couldn't change that: only workspace admins can do that."},
		"a store error is a 500": {payload: emojiAction(actionEmojiInput, ":tada:", "5", false), err: errors.New("dynamo down"), wantCode: http.StatusInternalServerError,
			wantCalls: "set workspace=false tier=5 :tada:"},
		"reset the workspace": {payload: emojiAction(actionEmojiReset, "", "", true), wantCode: http.StatusOK,
			wantCalls: "reset workspace=true,describe", wantNotice: "reset"},
		"picking a tier alone does nothing": {payload: emojiAction(actionEmojiTier, "", "5", false), wantCode: http.StatusOK},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			reactions := &recordingReactions{err: tc.err}
			p, posts := interactionsUnderTest(&recordingSettings{}, reactions, http.StatusOK)
			rec := sendInteraction(p, interactionBody(tc.payload), true)
			if rec.Code != tc.wantCode {
				t.Fatalf("status %d, want %d", rec.Code, tc.wantCode)
			}
			if got := strings.Join(reactions.calls, ","); tc.wantCalls != "" && got != tc.wantCalls {
				t.Fatalf("calls %s, want %s", got, tc.wantCalls)
			}
			if tc.wantNotice == "" {
				if len(*posts) != 0 {
					t.Fatalf("expected no panel refresh, got %+v", *posts)
				}
				return
			}
			if len(*posts) != 1 || (*posts)[0].text != tc.wantNotice {
				t.Fatalf("panel refresh %+v, want notice %q", *posts, tc.wantNotice)
			}
		})
	}
}
