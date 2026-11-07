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

// ReactionTierProvider returns the effective emoji -> karma tier map for a channel.
type ReactionTierProvider interface {
	ReactionTiers(ctx context.Context, teamID, channelID string) (map[string]int, error)
}

// ReactionStateStore records each reactor's tier emojis per message and returns
// the set as it was before the change.
type ReactionStateStore interface {
	AddReaction(ctx context.Context, teamID, channelID, messageTS, reactorID, emoji string) ([]string, error)
	RemoveReaction(ctx context.Context, teamID, channelID, messageTS, reactorID, emoji string) ([]string, error)
}

type EventsProcessor struct {
	signingSecret string
	karmaService  KarmaActionService
	settings      ChannelSettingsProvider
	userGroups    UserGroupMembersLister
	webClient     WebClient
	reactionTiers ReactionTierProvider
	reactionState ReactionStateStore
}

// WithReactions enables emoji-reaction karma. Without it reaction events are ignored.
func (p *EventsProcessor) WithReactions(tiers ReactionTierProvider, state ReactionStateStore) *EventsProcessor {
	p.reactionTiers = tiers
	p.reactionState = state
	return p
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

	if envelope.Type == "event_callback" && isReactionEvent(envelope.Event.Type) {
		if err := p.handleReaction(r.Context(), envelope); err != nil {
			http.Error(w, "failed to apply reaction karma", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
		return
	}

	if envelope.Type != "event_callback" || !isSupportedKarmaEventType(envelope.Event.Type) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
		return
	}

	// Only plain "message sent" events have no subtype (Slack omits it). Anything else
	// (message_changed, bot_message, channel_join, file_share, …) must not apply karma.
	if envelope.Event.Subtype != "" || envelope.Event.BotID != "" {
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

	segments := parser.DedupeKarmaSegments(parser.ParseKarmaSegments(stripBotMention(envelope)))
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

// stripBotMention removes the bot's own <@BOT> so "@sam @plusplus ++" credits sam, not the bot.
func stripBotMention(envelope EventEnvelope) string {
	botID := botUserIDFromAuthorizations(envelope.Authorizations)
	if botID == "" {
		return envelope.Event.Text
	}
	return strings.ReplaceAll(envelope.Event.Text, "<@"+botID+">", "")
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

	// Record the group-level action and pick up any "karma train" snark. Done
	// once per award (not per member) so repeated awards to the same group chain.
	warLine := p.observeGroupAward(ctx, envelope.TeamID, envelope.Event.User, seg.SubteamID, seg.SymbolRun, snarkLevel)

	if len(lines) == 0 {
		return nil
	}
	if warLine != "" {
		lines = append(lines, warLine)
	}
	return p.webClient.PostMessage(ctx, envelope.TeamID, envelope.Event.Channel, strings.Join(lines, "\n"), threadTS)
}

// groupAwardObserver is satisfied by *domain.KarmaService; other KarmaActionService
// implementations (tests, noop) simply don't trigger train detection.
type groupAwardObserver interface {
	ObserveGroupAward(ctx context.Context, teamID, actorID, groupID string, sign, snarkLevel int) (string, error)
}

func (p *EventsProcessor) observeGroupAward(ctx context.Context, teamID, actorID, groupID, symbolRun string, snarkLevel int) string {
	obs, ok := p.karmaService.(groupAwardObserver)
	if !ok {
		return ""
	}
	sign := domain.SymbolRunSign(symbolRun)
	if sign == 0 {
		return ""
	}
	line, _ := obs.ObserveGroupAward(ctx, teamID, actorID, groupID, sign, snarkLevel)
	return line
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

func isReactionEvent(eventType string) bool {
	return eventType == "reaction_added" || eventType == "reaction_removed"
}

// handleReaction turns a tier emoji reaction into karma for the message author.
// Each reactor's award on a message is their strongest positive tier minus their
// strongest negative tier; adding or removing a reaction applies the difference.
// Only additions that change the award reply (in the message's thread); self-reactions
// and removals stay silent.
func (p *EventsProcessor) handleReaction(ctx context.Context, envelope EventEnvelope) error {
	ev := envelope.Event
	if p.reactionTiers == nil || p.reactionState == nil || ev.Item.Type != "message" {
		return nil
	}
	if ev.ItemUser == "" || ev.ItemUser == ev.User || ev.ItemUser == botUserIDFromAuthorizations(envelope.Authorizations) {
		return nil
	}

	tiers, err := p.reactionTiers.ReactionTiers(ctx, envelope.TeamID, ev.Item.Channel)
	if err != nil {
		return err
	}
	// ponytail: tiers resolve with the current map, so editing the map between an add and
	// its removal can make the undo slightly off. Store the tier per emoji if that matters.
	emoji := domain.NormalizeEmoji(ev.Reaction)
	if _, ok := tiers[emoji]; !ok {
		return nil
	}

	added := ev.Type == "reaction_added"
	var before []string
	if added {
		before, err = p.reactionState.AddReaction(ctx, envelope.TeamID, ev.Item.Channel, ev.Item.TS, ev.User, emoji)
	} else {
		before, err = p.reactionState.RemoveReaction(ctx, envelope.TeamID, ev.Item.Channel, ev.Item.TS, ev.User, emoji)
	}
	if err != nil {
		return err
	}

	after := make([]string, 0, len(before)+1)
	for _, e := range before {
		if e != emoji {
			after = append(after, e)
		}
	}
	if added {
		after = append(after, emoji)
	}

	delta := domain.ReactionEffect(tiers, after) - domain.ReactionEffect(tiers, before)
	if delta == 0 {
		return nil
	}

	snarkLevel := domain.DefaultSnarkLevel
	if p.settings != nil {
		if _, level, err := p.settings.GetChannelSettings(ctx, envelope.TeamID, ev.Item.Channel); err == nil {
			snarkLevel = domain.NormalizeSnarkLevel(level)
		}
	}

	symbol := "+"
	if delta < 0 {
		symbol = "-"
	}
	result, err := p.karmaService.HandleAction(ctx, domain.KarmaAction{
		TeamID:       envelope.TeamID,
		ActorUserID:  ev.User,
		TargetUserID: ev.ItemUser,
		TargetHandle: "<@" + ev.ItemUser + ">",
		SymbolRun:    strings.Repeat(symbol, abs(delta)+1), // a run of n+1 symbols moves karma by n
		SnarkLevel:   snarkLevel,
	})
	if err != nil {
		return err
	}
	if !added || result.Message == "" {
		return nil
	}
	return p.webClient.PostMessage(ctx, envelope.TeamID, ev.Item.Channel, result.Message, ev.Item.TS)
}
