package slack

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"plusplus/internal/domain"
)

type SettingsInteractionService interface {
	SetSnarkLevel(ctx context.Context, teamID string, channelID string, actorUserID string, level int) (string, error)
}

type InteractionsProcessor struct {
	signingSecret   string
	settingsService SettingsInteractionService
}

func NewInteractionsProcessor(signingSecret string, settings SettingsInteractionService) *InteractionsProcessor {
	return &InteractionsProcessor{
		signingSecret:   signingSecret,
		settingsService: settings,
	}
}

func (p *InteractionsProcessor) HandleInteraction(w http.ResponseWriter, r *http.Request) {
	p.processInteraction(w, r)
}

func (p *InteractionsProcessor) processInteraction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read interaction payload", http.StatusBadRequest)
		return
	}

	timestamp := r.Header.Get("X-Slack-Request-Timestamp")
	signature := r.Header.Get("X-Slack-Signature")
	if err := VerifyRequestSignature(p.signingSecret, timestamp, signature, body); err != nil {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	values, err := url.ParseQuery(string(body))
	if err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	payloadStr := values.Get("payload")
	if payloadStr == "" {
		http.Error(w, "missing payload", http.StatusBadRequest)
		return
	}

	var payload slackInteractionPayload
	if err := json.Unmarshal([]byte(payloadStr), &payload); err != nil {
		http.Error(w, "invalid payload json", http.StatusBadRequest)
		return
	}

	if payload.Type != "block_actions" {
		w.WriteHeader(http.StatusOK)
		return
	}

	var action *slackBlockAction
	for i := range payload.Actions {
		if payload.Actions[i].ActionID == "snark_level_select" {
			action = &payload.Actions[i]
			break
		}
	}
	if action == nil || action.SelectedOption.Value == "" {
		w.WriteHeader(http.StatusOK)
		return
	}

	level, err := strconv.Atoi(action.SelectedOption.Value)
	if err != nil || level < domain.MinSnarkLevel || level > domain.MaxSnarkLevel {
		writeInteractionJSON(w, http.StatusOK, interactionResponse{
			ResponseType: "ephemeral",
			Text:         "Invalid snark level.",
		})
		return
	}

	msg, err := p.settingsService.SetSnarkLevel(
		r.Context(),
		payload.Team.ID,
		payload.Channel.ID,
		payload.User.ID,
		level,
	)
	if err != nil {
		http.Error(w, "failed to update settings", http.StatusInternalServerError)
		return
	}

	writeInteractionJSON(w, http.StatusOK, interactionResponse{
		ResponseType: "ephemeral",
		Text:         msg,
	})
}

type slackInteractionPayload struct {
	Type    string             `json:"type"`
	User    slackEntityID      `json:"user"`
	Team    slackEntityID      `json:"team"`
	Channel slackEntityID      `json:"channel"`
	Actions []slackBlockAction `json:"actions"`
}

type slackEntityID struct {
	ID string `json:"id"`
}

type slackBlockAction struct {
	ActionID       string `json:"action_id"`
	SelectedOption struct {
		Value string `json:"value"`
	} `json:"selected_option"`
}

type interactionResponse struct {
	ResponseType string `json:"response_type"`
	Text         string `json:"text"`
}

func writeInteractionJSON(w http.ResponseWriter, status int, payload interactionResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
