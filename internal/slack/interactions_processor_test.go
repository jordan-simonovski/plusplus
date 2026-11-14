package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type recordingSnarkSetter struct {
	calls []string // "team/channel/user/level"
	err   error
}

func (r *recordingSnarkSetter) SetSnarkLevel(_ context.Context, team, channel, user string, level int) (string, error) {
	r.calls = append(r.calls, strings.Join([]string{team, channel, user, string(rune('0' + level))}, "/"))
	if r.err != nil {
		return "", r.err
	}
	return "Snark level updated.", nil
}

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

func snarkSelect(value string) string {
	return `{"type":"block_actions","user":{"id":"U1"},"team":{"id":"T1"},"channel":{"id":"C1"},
		"actions":[{"action_id":"snark_level_select","selected_option":{"value":"` + value + `"}}]}`
}

func TestInteractionsSnarkLevel(t *testing.T) {
	cases := map[string]struct {
		body      []byte
		sign      bool
		storeErr  error
		wantCode  int
		wantText  string
		wantCalls []string
	}{
		"unsigned":               {body: interactionBody(snarkSelect("7")), wantCode: http.StatusUnauthorized},
		"no payload field":       {body: interactionBody(""), sign: true, wantCode: http.StatusBadRequest},
		"not block_actions":      {body: interactionBody(`{"type":"view_submission"}`), sign: true, wantCode: http.StatusOK},
		"no snark select action": {body: interactionBody(`{"type":"block_actions","actions":[{"action_id":"other"}]}`), sign: true, wantCode: http.StatusOK},
		"level 0":                {body: interactionBody(snarkSelect("0")), sign: true, wantCode: http.StatusOK, wantText: "Invalid snark level."},
		"level 11":               {body: interactionBody(snarkSelect("11")), sign: true, wantCode: http.StatusOK, wantText: "Invalid snark level."},
		"level abc":              {body: interactionBody(snarkSelect("abc")), sign: true, wantCode: http.StatusOK, wantText: "Invalid snark level."},
		"valid level":            {body: interactionBody(snarkSelect("7")), sign: true, wantCode: http.StatusOK, wantText: "Snark level updated.", wantCalls: []string{"T1/C1/U1/7"}},
		"service error is a 500": {body: interactionBody(snarkSelect("7")), sign: true, storeErr: errors.New("dynamo down"), wantCode: http.StatusInternalServerError, wantCalls: []string{"T1/C1/U1/7"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			svc := &recordingSnarkSetter{err: tc.storeErr}
			rec := sendInteraction(NewInteractionsProcessor(svc), tc.body, tc.sign)
			if rec.Code != tc.wantCode {
				t.Fatalf("status %d, want %d (%s)", rec.Code, tc.wantCode, rec.Body.String())
			}
			if strings.Join(svc.calls, ",") != strings.Join(tc.wantCalls, ",") {
				t.Fatalf("service calls %v, want %v", svc.calls, tc.wantCalls)
			}
			if tc.wantText != "" {
				var resp struct {
					Text string `json:"text"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil || resp.Text != tc.wantText {
					t.Fatalf("response %q, want text %q", rec.Body.String(), tc.wantText)
				}
			}
		})
	}
}
