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
	"strconv"
	"strings"
	"testing"
	"time"

	"plusplus/internal/domain"
)

func TestCommandsProcessorLeaderboard(t *testing.T) {
	processor := NewCommandsProcessor("secret", fakeLeaderboardService{}, fakeSettingsCommandService{}, "test")
	body := []byte("team_id=T1&channel_id=C1&user_id=U1&command=%2Fplusplus-leaderboard&text=")
	req := httptest.NewRequest(http.MethodPost, "/slack/commands", bytes.NewReader(body))
	addSignedHeaders(req, "secret", body)

	rec := httptest.NewRecorder()
	processor.ProcessCommand(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}

	var response MessageResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.ResponseType != "in_channel" {
		t.Fatalf("unexpected response type: %s", response.ResponseType)
	}
}

func TestCommandsProcessorSettingsUsage(t *testing.T) {
	processor := NewCommandsProcessor("secret", fakeLeaderboardService{}, fakeSettingsCommandService{}, "test")
	body := []byte("team_id=T1&channel_id=C1&user_id=U1&command=%2Fplusplus-settings&text=bad")
	req := httptest.NewRequest(http.MethodPost, "/slack/commands", bytes.NewReader(body))
	addSignedHeaders(req, "secret", body)

	rec := httptest.NewRecorder()
	processor.ProcessCommand(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}

	var response MessageResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.ResponseType != "ephemeral" {
		t.Fatalf("unexpected response type: %s", response.ResponseType)
	}
}

func TestCommandsProcessorHelpAndUnknownInput(t *testing.T) {
	cases := []string{
		"command=%2Fplusplus-leaderboard&text=help",
		"command=%2Fplusplus-leaderboard&text=whatever",
		"command=%2Fplusplus-settings&text=help",
		"command=%2Fplusplus-settings&text=reply_mode+sideways",
		"command=%2Fplusplus-nope&text=",
	}
	legacy := []string{
		"command=%2Fleaderboard&text=help",
		"command=%2Fsettings&text=bad",
	}
	processor := NewCommandsProcessor("secret", fakeLeaderboardService{}, fakeSettingsCommandService{}, "test")
	for _, c := range cases {
		body := []byte("team_id=T1&channel_id=C1&user_id=U1&" + c)
		req := httptest.NewRequest(http.MethodPost, "/slack/commands", bytes.NewReader(body))
		addSignedHeaders(req, "secret", body)
		rec := httptest.NewRecorder()
		processor.ProcessCommand(rec, req)

		var response MessageResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("%s: decode response: %v", c, err)
		}
		if response.ResponseType != "ephemeral" || response.Text != usageText {
			t.Fatalf("%s: expected ephemeral usage text, got %+v", c, response)
		}
	}
	for _, c := range legacy {
		body := []byte("team_id=T1&channel_id=C1&user_id=U1&" + c)
		req := httptest.NewRequest(http.MethodPost, "/slack/commands", bytes.NewReader(body))
		addSignedHeaders(req, "secret", body)
		rec := httptest.NewRecorder()
		processor.ProcessCommand(rec, req)

		var response MessageResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("%s: decode response: %v", c, err)
		}
		if !strings.Contains(response.Text, "`/leaderboard`") || strings.Contains(response.Text, "/plusplus-") {
			t.Fatalf("%s: expected legacy command names in usage, got %q", c, response.Text)
		}
	}
}

func TestCommandsProcessorLegacyLeaderboardStillWorks(t *testing.T) {
	processor := NewCommandsProcessor("secret", fakeLeaderboardService{}, fakeSettingsCommandService{}, "test")
	body := []byte("team_id=T1&channel_id=C1&user_id=U1&command=%2Fleaderboard&text=")
	req := httptest.NewRequest(http.MethodPost, "/slack/commands", bytes.NewReader(body))
	addSignedHeaders(req, "secret", body)
	rec := httptest.NewRecorder()
	processor.ProcessCommand(rec, req)

	var response MessageResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.ResponseType != "in_channel" || response.Text != "leaderboard" {
		t.Fatalf("expected legacy /leaderboard to return the leaderboard, got %+v", response)
	}
}

type fakeLeaderboardService struct{}

func (s fakeLeaderboardService) HandleLeaderboard(_ context.Context, _ domain.LeaderboardRequest) (domain.KarmaResult, error) {
	return domain.KarmaResult{Message: "leaderboard"}, nil
}

type fakeSettingsCommandService struct{}

func (s fakeSettingsCommandService) SetReplyMode(_ context.Context, _ string, _ string, _ string, mode ReplyMode) (string, error) {
	return "reply mode set to " + string(mode), nil
}

func (s fakeSettingsCommandService) SetSnarkLevel(_ context.Context, _ string, _ string, _ string, level int) (string, error) {
	return "snark level set to " + strconv.Itoa(level), nil
}

func (s fakeSettingsCommandService) GetSnarkLevel(_ context.Context, _ string, _ string) (int, error) {
	return 5, nil
}

func addSignedHeaders(req *http.Request, secret string, body []byte) {
	ts := fmt.Sprintf("%d", time.Now().Unix())
	req.Header.Set("X-Slack-Request-Timestamp", ts)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("v0:" + ts + ":"))
	mac.Write(body)
	req.Header.Set("X-Slack-Signature", "v0="+hex.EncodeToString(mac.Sum(nil)))
}
