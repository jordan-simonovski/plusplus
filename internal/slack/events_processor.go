package slack

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"plusplus/internal/domain"
	"plusplus/internal/parser"
)

type ReplyMode string

const (
	ReplyModeThread  ReplyMode = "thread"
	ReplyModeChannel ReplyMode = "channel"
)

type KarmaActionService interface {
	HandleAction(ctx context.Context, action domain.KarmaAction) (domain.KarmaResult, error)
}

type ReplyModeProvider interface {
	GetReplyMode(ctx context.Context, teamID string, channelID string) (ReplyMode, error)
}

type WebClient interface {
	PostMessage(ctx context.Context, channelID string, text string, threadTS string) error
}

type EventsProcessor struct {
	signingSecret string
	karmaService  KarmaActionService
	settings      ReplyModeProvider
	webClient     WebClient
}

func NewEventsProcessor(
	signingSecret string,
	karmaService KarmaActionService,
	settings ReplyModeProvider,
	webClient WebClient,
) *EventsProcessor {
	return &EventsProcessor{
		signingSecret: signingSecret,
		karmaService:  karmaService,
		settings:      settings,
		webClient:     webClient,
	}
}

func (p *EventsProcessor) ProcessEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	timestamp := r.Header.Get("X-Slack-Request-Timestamp")
	signature := r.Header.Get("X-Slack-Signature")
	if err := VerifyRequestSignature(p.signingSecret, timestamp, signature, body); err != nil {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	var envelope EventEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	if envelope.Type == "url_verification" {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"challenge": envelope.Challenge})
		return
	}

	if envelope.Type != "event_callback" || envelope.Event.Type != "app_mention" {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
		return
	}

	if envelope.Event.Subtype == "bot_message" || envelope.Event.BotID != "" {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
		return
	}

	parsed, ok := parser.ParseMentionAction(envelope.Event.Text)
	if !ok {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
		return
	}

	result, err := p.karmaService.HandleAction(r.Context(), domain.KarmaAction{
		TeamID:       envelope.TeamID,
		ActorUserID:  envelope.Event.User,
		TargetUserID: parsed.TargetUserID,
		TargetHandle: "<@" + parsed.TargetUserID + ">",
		SymbolRun:    parsed.SymbolRun,
	})
	if err != nil {
		http.Error(w, "failed to apply karma", http.StatusInternalServerError)
		return
	}

	threadTS := p.resolveThreadTS(r.Context(), envelope)
	if err := p.webClient.PostMessage(r.Context(), envelope.Event.Channel, result.Message, threadTS); err != nil {
		http.Error(w, "failed to post message", http.StatusBadGateway)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (p *EventsProcessor) resolveThreadTS(ctx context.Context, envelope EventEnvelope) string {
	if p.settings == nil {
		return defaultThreadTS(envelope.Event)
	}

	mode, err := p.settings.GetReplyMode(ctx, envelope.TeamID, envelope.Event.Channel)
	if err != nil {
		return defaultThreadTS(envelope.Event)
	}

	if mode == ReplyModeChannel {
		return ""
	}

	return defaultThreadTS(envelope.Event)
}

func defaultThreadTS(event SlackEvent) string {
	if event.ThreadTS != "" {
		return event.ThreadTS
	}
	return event.EventTS
}
