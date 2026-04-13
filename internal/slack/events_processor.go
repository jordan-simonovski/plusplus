package slack

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"plusplus/internal/domain"
	"plusplus/internal/parser"
	"strings"
)

type KarmaActionService interface {
	HandleAction(ctx context.Context, action domain.KarmaAction) (domain.KarmaResult, error)
}

type ChannelSettingsProvider interface {
	GetChannelSettings(ctx context.Context, teamID string, channelID string) (ReplyMode, int, error)
}

type WebClient interface {
	PostMessage(ctx context.Context, teamID string, channelID string, text string, threadTS string) error
}

// UserGroupMembersLister resolves Slack user group (subteam) IDs to member user IDs (usergroups.users.list).
type UserGroupMembersLister interface {
	ListUserGroupMembers(ctx context.Context, teamID, userGroupID string) ([]string, error)
}

type EventsProcessor struct {
	signingSecret string
	karmaService  KarmaActionService
	settings      ChannelSettingsProvider
	userGroups    UserGroupMembersLister
	webClient     WebClient
}

func NewEventsProcessor(
	signingSecret string,
	karmaService KarmaActionService,
	settings ChannelSettingsProvider,
	userGroups UserGroupMembersLister,
	webClient WebClient,
) *EventsProcessor {
	return &EventsProcessor{
		signingSecret: signingSecret,
		karmaService:  karmaService,
		settings:      settings,
		userGroups:    userGroups,
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

	if envelope.Type != "event_callback" || !isSupportedKarmaEventType(envelope.Event.Type) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
		return
	}

	if envelope.Event.Subtype == "bot_message" || envelope.Event.BotID != "" {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
		return
	}

	// Slack delivers both app_mention and message for the same post when the bot is @-mentioned.
	// Process app_mention only; otherwise karma is applied twice.
	if envelope.Event.Type == "message" && messageEventDuplicatesAppMention(envelope) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
		return
	}

	segments := parser.DedupeKarmaSegments(parser.ParseKarmaSegments(envelope.Event.Text))
	if len(segments) == 0 {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
		return
	}

	threadTS, snarkLevel := p.resolveChannelContext(r.Context(), envelope)

	for _, seg := range segments {
		switch seg.Kind {
		case parser.KarmaSegmentUser:
			result, err := p.karmaService.HandleAction(r.Context(), domain.KarmaAction{
				TeamID:       envelope.TeamID,
				ActorUserID:  envelope.Event.User,
				TargetUserID: seg.UserID,
				TargetHandle: "<@" + seg.UserID + ">",
				SymbolRun:    seg.SymbolRun,
				SnarkLevel:   snarkLevel,
			})
			if err != nil {
				http.Error(w, "failed to apply karma", http.StatusInternalServerError)
				return
			}

			if result.Message != "" {
				if err := p.webClient.PostMessage(r.Context(), envelope.TeamID, envelope.Event.Channel, result.Message, threadTS); err != nil {
					http.Error(w, "failed to post message", http.StatusBadGateway)
					return
				}
			}

		case parser.KarmaSegmentSubteam:
			if err := p.handleSubteamKarma(r, envelope, seg, threadTS, snarkLevel); err != nil {
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
		}
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (p *EventsProcessor) resolveChannelContext(ctx context.Context, envelope EventEnvelope) (threadTS string, snarkLevel int) {
	snarkLevel = domain.DefaultSnarkLevel
	if p.settings == nil {
		return defaultThreadTS(envelope.Event), snarkLevel
	}

	mode, level, err := p.settings.GetChannelSettings(ctx, envelope.TeamID, envelope.Event.Channel)
	if err != nil {
		return defaultThreadTS(envelope.Event), snarkLevel
	}

	snarkLevel = domain.NormalizeSnarkLevel(level)

	if mode == ReplyModeChannel {
		return "", snarkLevel
	}

	return defaultThreadTS(envelope.Event), snarkLevel
}

func defaultThreadTS(event SlackEvent) string {
	if event.ThreadTS != "" {
		return event.ThreadTS
	}
	return event.EventTS
}

func isSupportedKarmaEventType(eventType string) bool {
	return eventType == "app_mention" || eventType == "message"
}

func botUserIDFromAuthorizations(auth []SlackAuthorization) string {
	for _, a := range auth {
		if a.IsBot && a.UserID != "" {
			return a.UserID
		}
	}
	if len(auth) > 0 && auth[0].UserID != "" {
		return auth[0].UserID
	}
	return ""
}

func messageEventDuplicatesAppMention(envelope EventEnvelope) bool {
	botID := botUserIDFromAuthorizations(envelope.Authorizations)
	if botID == "" {
		return false
	}
	return strings.Contains(envelope.Event.Text, "<@"+botID+">")
}

func (p *EventsProcessor) handleSubteamKarma(
	r *http.Request,
	envelope EventEnvelope,
	seg parser.KarmaSegment,
	threadTS string,
	snarkLevel int,
) error {
	ctx := r.Context()
	if p.userGroups == nil {
		return p.webClient.PostMessage(ctx, envelope.TeamID, envelope.Event.Channel, "Could not resolve user groups (not configured).", threadTS)
	}

	members, err := p.userGroups.ListUserGroupMembers(ctx, envelope.TeamID, seg.SubteamID)
	if err != nil {
		return p.webClient.PostMessage(ctx, envelope.TeamID, envelope.Event.Channel, "Could not load members for that user group.", threadTS)
	}

	members = dedupePreserveOrder(members)
	if len(members) == 0 {
		return p.webClient.PostMessage(ctx, envelope.TeamID, envelope.Event.Channel, "That user group has no members.", threadTS)
	}

	var lines []string
	for _, uid := range members {
		result, err := p.karmaService.HandleAction(ctx, domain.KarmaAction{
			TeamID:         envelope.TeamID,
			ActorUserID:    envelope.Event.User,
			TargetUserID:   uid,
			TargetHandle:   "<@" + uid + ">",
			SymbolRun:      seg.SymbolRun,
			SnarkLevel:     snarkLevel,
			GroupBroadcast: true,
		})
		if err != nil {
			return err
		}
		if result.Message != "" {
			lines = append(lines, result.Message)
		}
	}

	if len(lines) == 0 {
		return nil
	}
	return p.webClient.PostMessage(ctx, envelope.TeamID, envelope.Event.Channel, strings.Join(lines, "\n"), threadTS)
}

func dedupePreserveOrder(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
