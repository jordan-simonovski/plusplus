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
	"testing"
	"time"

	"plusplus/internal/domain"
)

func TestCommandsProcessorLeaderboard(t *testing.T) {
	processor := NewCommandsProcessor("secret", fakeLeaderboardService{}, fakeSettingsCommandService{})
	body := []byte("team_id=T1&channel_id=C1&user_id=U1&command=%2Fleaderboard&text=")
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
	processor := NewCommandsProcessor("secret", fakeLeaderboardService{}, fakeSettingsCommandService{})
	body := []byte("team_id=T1&channel_id=C1&user_id=U1&command=%2Fsettings&text=bad")
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
