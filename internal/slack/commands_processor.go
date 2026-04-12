package slack

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"plusplus/internal/domain"
)

type LeaderboardService interface {
	HandleLeaderboard(ctx context.Context, request domain.LeaderboardRequest) (domain.KarmaResult, error)
}

type SettingsCommandService interface {
	SetReplyMode(ctx context.Context, teamID string, channelID string, actorUserID string, mode ReplyMode) (string, error)
}

type CommandsProcessor struct {
	signingSecret   string
	leaderboard     LeaderboardService
	settingsService SettingsCommandService
}

func NewCommandsProcessor(signingSecret string, leaderboard LeaderboardService, settingsService SettingsCommandService) *CommandsProcessor {
	return &CommandsProcessor{
		signingSecret:   signingSecret,
		leaderboard:     leaderboard,
		settingsService: settingsService,
	}
}

func (p *CommandsProcessor) ProcessCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read command payload", http.StatusBadRequest)
		return
	}

	timestamp := r.Header.Get("X-Slack-Request-Timestamp")
	signature := r.Header.Get("X-Slack-Signature")
	if err := VerifyRequestSignature(p.signingSecret, timestamp, signature, body); err != nil {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	payload, err := parseSlashPayload(body)
	if err != nil {
		http.Error(w, "invalid command payload", http.StatusBadRequest)
		return
	}

	switch payload.Command {
	case "/leaderboard":
		p.respondLeaderboard(w, r, payload)
	case "/settings":
		p.respondSettings(w, r, payload)
	default:
		writeJSON(w, http.StatusOK, MessageResponse{
			ResponseType: "ephemeral",
			Text:         "Unsupported command. Use /leaderboard or /settings.",
		})
	}
}

func parseSlashPayload(body []byte) (SlashCommandPayload, error) {
	values, err := url.ParseQuery(string(body))
	if err != nil {
		return SlashCommandPayload{}, err
	}

	return SlashCommandPayload{
		TeamID:      values.Get("team_id"),
		ChannelID:   values.Get("channel_id"),
		UserID:      values.Get("user_id"),
		Command:     values.Get("command"),
		Text:        strings.TrimSpace(values.Get("text")),
		ResponseURL: values.Get("response_url"),
		TriggerID:   values.Get("trigger_id"),
	}, nil
}

func (p *CommandsProcessor) respondLeaderboard(w http.ResponseWriter, r *http.Request, payload SlashCommandPayload) {
	result, err := p.leaderboard.HandleLeaderboard(r.Context(), domain.LeaderboardRequest{
		TeamID: payload.TeamID,
	})
	if err != nil {
		http.Error(w, "failed to build leaderboard", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, MessageResponse{
		ResponseType: "in_channel",
		Text:         result.Message,
	})
}

func (p *CommandsProcessor) respondSettings(w http.ResponseWriter, r *http.Request, payload SlashCommandPayload) {
	parts := strings.Fields(payload.Text)
	if len(parts) != 2 || parts[0] != "reply_mode" {
		writeJSON(w, http.StatusOK, MessageResponse{
			ResponseType: "ephemeral",
			Text:         "Usage: /settings reply_mode thread|channel",
		})
		return
	}

	var mode ReplyMode
	switch parts[1] {
	case string(ReplyModeThread):
		mode = ReplyModeThread
	case string(ReplyModeChannel):
		mode = ReplyModeChannel
	default:
		writeJSON(w, http.StatusOK, MessageResponse{
			ResponseType: "ephemeral",
			Text:         "Usage: /settings reply_mode thread|channel",
		})
		return
	}

	msg, err := p.settingsService.SetReplyMode(r.Context(), payload.TeamID, payload.ChannelID, payload.UserID, mode)
	if err != nil {
		http.Error(w, "failed to update settings", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, MessageResponse{
		ResponseType: "ephemeral",
		Text:         msg,
	})
}

func writeJSON(w http.ResponseWriter, status int, payload MessageResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
