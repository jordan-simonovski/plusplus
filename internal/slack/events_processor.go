package slack

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"plusplus/internal/domain"
	"plusplus/internal/parser"
	"strings"
	"time"
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

// ReactionStateStore records each reactor's tier emojis per message, with the tier
// each was worth when added, and returns the emoji -> tier map as it was before.
// It also remembers who each typed karma message credited, so reactions to
// "@plusplus @sam ++" go to sam rather than the poster.
type ReactionStateStore interface {
	AddReaction(ctx context.Context, teamID, channelID, messageTS, reactorID, emoji string, tier int) (map[string]int, error)
	RemoveReaction(ctx context.Context, teamID, channelID, messageTS, reactorID, emoji string) (map[string]int, error)
	RecordKarmaTargets(ctx context.Context, teamID, channelID, messageTS string, targets []string) error
	KarmaTargets(ctx context.Context, teamID, channelID, messageTS string) (targets []string, ok bool, err error)
}

// InstallationRemover forgets a workspace's bot token when the app is uninstalled.
type InstallationRemover interface {
	RemoveInstallation(ctx context.Context, teamID string) error
}

// EventClaimer makes typed karma at-most-once per Slack event: Slack re-sends an event when
// it gets no 2xx in three seconds, and a re-send must not apply the same karma again.
type EventClaimer interface {
	// ClaimEvent is true for the first delivery of an event (or after ReleaseEvent).
	ClaimEvent(ctx context.Context, teamID, eventID string) (bool, error)
	ReleaseEvent(ctx context.Context, teamID, eventID string) error
}

type EventsProcessor struct {
	events        EventClaimer
	installs      InstallationRemover
	signingSecret string
	karmaService  KarmaActionService
	settings      ChannelSettingsProvider
	userGroups    UserGroupMembersLister
	webClient     WebClient
	reactionTiers ReactionTierProvider
	reactionState ReactionStateStore
	reactionCap   int
}

// WithReactions enables emoji-reaction karma. Without it reaction events are ignored.
// maxKarmaPerAction caps each reaction tier (see domain.ClampReactionTier).
func (p *EventsProcessor) WithReactions(tiers ReactionTierProvider, state ReactionStateStore, maxKarmaPerAction int) *EventsProcessor {
	p.reactionTiers = tiers
	p.reactionState = state
	p.reactionCap = maxKarmaPerAction
	return p
}

// WithInstallationRemover drops a workspace's stored bot token on app_uninstalled and on
// tokens_revoked for the bot. Without it (single-workspace mode) those events are ignored.
func (p *EventsProcessor) WithInstallationRemover(installs InstallationRemover) *EventsProcessor {
	p.installs = installs
	return p
}

// WithEventClaims makes typed karma safe for Slack to retry. Without it every delivery is
// processed.
func (p *EventsProcessor) WithEventClaims(events EventClaimer) *EventsProcessor {
	p.events = events
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

	if envelope.Type == "event_callback" && uninstallsBot(envelope.Event) {
		// Removing twice is harmless, so an error is safe for Slack to retry.
		if p.installs != nil {
			if err := p.installs.RemoveInstallation(r.Context(), envelope.TeamID); err != nil {
				slog.Error("remove installation failed", "team", envelope.TeamID, "kind", envelope.Event.Type, "err", err)
				http.Error(w, "failed to remove installation", http.StatusInternalServerError)
				return
			}
			slog.Info("installation removed", "team", envelope.TeamID, "kind", envelope.Event.Type)
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
		return
	}

	if envelope.Type == "event_callback" && isReactionEvent(envelope.Event.Type) {
		// A timeout retry means the first delivery is still running (it doesn't stop when
		// Slack hangs up), so processing it again would double-apply or resurrect a
		// reaction that was removed in between. Error retries are processed normally.
		if r.Header.Get("X-Slack-Retry-Reason") == "http_timeout" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
			return
		}
		// Finish the work even if Slack disconnects at its 3s deadline.
		ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 15*time.Second)
		defer cancel()
		if err := p.handleReaction(ctx, envelope); err != nil {
			slog.Error("reaction karma failed", "team", envelope.TeamID, "kind", envelope.Event.Type, "err", err)
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

	// Finish the work even if Slack disconnects at its 3s deadline.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 15*time.Second)
	defer cancel()

	// Claim the event before writing anything. Once karma may have been written the claim
	// is kept even on failure: a dropped award is visible and can be retyped, a doubled one
	// is not. Only failures before the first HandleAction release it.
	claimed := false
	if p.events != nil && envelope.EventID != "" {
		ok, err := p.events.ClaimEvent(ctx, envelope.TeamID, envelope.EventID)
		if err != nil {
			slog.Error("claim event failed", "team", envelope.TeamID, "kind", envelope.Event.Type, "err", err)
			http.Error(w, "failed to claim event", http.StatusInternalServerError)
			return
		}
		if !ok {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
			return
		}
		claimed = true
	}

	// Recorded before any karma is applied, so failing here is safe for Slack to retry.
	if p.reactionState != nil {
		var targets []string
		for _, seg := range segments {
			if seg.Kind == parser.KarmaSegmentUser {
				targets = append(targets, seg.UserID)
			}
		}
		if err := p.reactionState.RecordKarmaTargets(ctx, envelope.TeamID, envelope.Event.Channel, messageTS(envelope.Event), targets); err != nil {
			slog.Error("record karma message failed", "team", envelope.TeamID, "kind", envelope.Event.Type, "err", err)
			if claimed {
				p.releaseEvent(ctx, envelope)
			}
			http.Error(w, "failed to record karma message", http.StatusInternalServerError)
			return
		}
	}

	threadTS, snarkLevel := p.resolveChannelContext(ctx, envelope)

	for _, seg := range segments {
		switch seg.Kind {
		case parser.KarmaSegmentUser:
			result, err := p.karmaService.HandleAction(ctx, domain.KarmaAction{
				TeamID:       envelope.TeamID,
				ActorUserID:  envelope.Event.User,
				TargetUserID: seg.UserID,
				TargetHandle: "<@" + seg.UserID + ">",
				SymbolRun:    seg.SymbolRun,
				SnarkLevel:   snarkLevel,
			})
			if err != nil {
				slog.Error("apply karma failed", "team", envelope.TeamID, "kind", envelope.Event.Type, "err", err)
				http.Error(w, "failed to apply karma", http.StatusInternalServerError)
				return
			}

			if result.Message != "" {
				if err := p.webClient.PostMessage(ctx, envelope.TeamID, envelope.Event.Channel, result.Message, threadTS); err != nil {
					slog.Error("post karma reply failed", "team", envelope.TeamID, "kind", envelope.Event.Type, "err", err)
					http.Error(w, "failed to post message", http.StatusBadGateway)
					return
				}
			}

		case parser.KarmaSegmentSubteam:
			if err := p.handleSubteamKarma(ctx, envelope, seg, threadTS, snarkLevel); err != nil {
				slog.Error("user group karma failed", "team", envelope.TeamID, "kind", envelope.Event.Type, "err", err)
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
		slog.Warn("channel settings unavailable, using defaults", "team", envelope.TeamID, "kind", envelope.Event.Type, "err", err)
		return defaultThreadTS(envelope.Event), snarkLevel
	}

	snarkLevel = domain.NormalizeSnarkLevel(level)

	if mode == ReplyModeChannel {
		return "", snarkLevel
	}

	return defaultThreadTS(envelope.Event), snarkLevel
}

// messageTS is the message's own ts, which is what reaction events reference.
func messageTS(event SlackEvent) string {
	if event.TS != "" {
		return event.TS
	}
	return event.EventTS
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
	ctx context.Context,
	envelope EventEnvelope,
	seg parser.KarmaSegment,
	threadTS string,
	snarkLevel int,
) error {
	if p.userGroups == nil {
		return p.webClient.PostMessage(ctx, envelope.TeamID, envelope.Event.Channel, "Could not resolve user groups (not configured).", threadTS)
	}

	members, err := p.userGroups.ListUserGroupMembers(ctx, envelope.TeamID, seg.SubteamID)
	if err != nil {
		slog.Warn("user group members unavailable", "team", envelope.TeamID, "kind", envelope.Event.Type, "err", err)
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
			// Members already credited keep their karma (the claim stops a retry from
			// re-applying it). Say so, or the user sees silence and retypes the award.
			if len(lines) > 0 {
				p.postBestEffort(ctx, envelope, strings.Join(append(lines, domain.GroupAwardStoppedNote), "\n"), threadTS)
			}
			return err
		}
		if result.Message != "" {
			lines = append(lines, result.Message)
		}
	}

	// Record the group-level action and pick up any "karma train" snark. Done
	// once per award (not per member) so repeated awards to the same group chain.
	warLine := p.observeGroupAward(ctx, envelope.TeamID, envelope.Event.Type, envelope.Event.User, seg.SubteamID, seg.SymbolRun, snarkLevel)

	if len(lines) == 0 {
		return nil
	}
	if warLine != "" {
		lines = append(lines, warLine)
	}
	return p.webClient.PostMessage(ctx, envelope.TeamID, envelope.Event.Channel, strings.Join(lines, "\n"), threadTS)
}

// releaseEvent lets Slack's retry process the event again. It runs on a short context of
// its own so it still works when the request's deadline is what failed; if it fails anyway
// the event stays claimed (dropped, never doubled) and the failure is logged.
func (p *EventsProcessor) releaseEvent(ctx context.Context, envelope EventEnvelope) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := p.events.ReleaseEvent(ctx, envelope.TeamID, envelope.EventID); err != nil {
		slog.Warn("could not release event claim", "team", envelope.TeamID, "kind", envelope.Event.Type, "event_id", envelope.EventID, "err", err)
	}
}

// postBestEffort posts on a short context of its own, so a reply still goes out when the
// request's deadline is what failed; errors are logged.
func (p *EventsProcessor) postBestEffort(ctx context.Context, envelope EventEnvelope, text, threadTS string) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := p.webClient.PostMessage(ctx, envelope.TeamID, envelope.Event.Channel, text, threadTS); err != nil {
		slog.Warn("could not post partial reply", "team", envelope.TeamID, "kind", envelope.Event.Type, "err", err)
	}
}

// groupAwardObserver is satisfied by *domain.KarmaService; other KarmaActionService
// implementations (tests, noop) simply don't trigger train detection.
type groupAwardObserver interface {
	ObserveGroupAward(ctx context.Context, teamID, actorID, groupID string, sign, snarkLevel int) (string, error)
}

func (p *EventsProcessor) observeGroupAward(ctx context.Context, teamID, kind, actorID, groupID, symbolRun string, snarkLevel int) string {
	obs, ok := p.karmaService.(groupAwardObserver)
	if !ok {
		return ""
	}
	sign := domain.SymbolRunSign(symbolRun)
	if sign == 0 {
		return ""
	}
	line, err := obs.ObserveGroupAward(ctx, teamID, actorID, groupID, sign, snarkLevel)
	if err != nil {
		slog.Warn("karma war store failed", "team", teamID, "kind", kind, "err", err)
	}
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

// handleReaction turns a tier emoji reaction into karma for the message author, or,
// on a typed karma message, for the people it credited (see reactionTargets).
// Each reactor's award on a message is their strongest positive tier plus their
// strongest negative tier; adding or removing a reaction applies the difference.
// Tiers are stored per reaction when added, so removals undo exactly what was given
// even if the emoji map changed since. Only additions that change the award reply (in
// the message's thread); self-reactions and removals stay silent.
func (p *EventsProcessor) handleReaction(ctx context.Context, envelope EventEnvelope) error {
	ev := envelope.Event
	if p.reactionTiers == nil || p.reactionState == nil || ev.Item.Type != "message" {
		return nil
	}
	if ev.ItemUser == "" || ev.ItemUser == ev.User || ev.ItemUser == botUserIDFromAuthorizations(envelope.Authorizations) {
		return nil
	}
	emoji := domain.NormalizeEmoji(ev.Reaction)
	if emoji == "" {
		return nil
	}
	team, channel, ts, reactor := envelope.TeamID, ev.Item.Channel, ev.Item.TS, ev.User

	added := ev.Type == "reaction_added"
	var before map[string]int
	var tier int
	if added {
		tiers, err := p.reactionTiers.ReactionTiers(ctx, team, channel)
		if err != nil {
			return err
		}
		mapped, ok := tiers[emoji]
		if !ok {
			return nil
		}
		tier = domain.ClampReactionTier(mapped, p.reactionCap)
		if before, err = p.reactionState.AddReaction(ctx, team, channel, ts, reactor, emoji, tier); err != nil {
			return err
		}
	} else {
		var err error
		if before, err = p.reactionState.RemoveReaction(ctx, team, channel, ts, reactor, emoji); err != nil {
			return err
		}
	}

	after := make(map[string]int, len(before)+1)
	for e, t := range before {
		after[e] = t
	}
	if added {
		after[emoji] = tier
	} else {
		delete(after, emoji)
	}
	delta := domain.NetReactionTier(after) - domain.NetReactionTier(before)
	if delta == 0 {
		return nil
	}

	targets, err := p.reactionTargets(ctx, team, channel, ts, ev.ItemUser, reactor)
	if err != nil {
		p.restoreReaction(ctx, ev.Type, team, channel, ts, reactor, emoji, before)
		return err
	}
	if len(targets) == 0 {
		return nil
	}

	snarkLevel := domain.DefaultSnarkLevel
	if p.settings != nil {
		if _, level, err := p.settings.GetChannelSettings(ctx, team, channel); err == nil {
			snarkLevel = domain.NormalizeSnarkLevel(level)
		} else {
			slog.Warn("channel settings unavailable, using defaults", "team", team, "kind", ev.Type, "err", err)
		}
	}

	var lines, applied []string
	for _, target := range targets {
		result, err := p.karmaService.HandleAction(ctx, domain.KarmaAction{
			TeamID:       team,
			ActorUserID:  reactor,
			TargetUserID: target,
			TargetHandle: "<@" + target + ">",
			SymbolRun:    symbolRun(delta),
			SnarkLevel:   snarkLevel,
			SkipWar:      !added || domain.NetReactionTier(before) != 0,
		})
		if err != nil {
			// Take back what this event already gave and put the reaction state back, so
			// Slack's retry applies the same delta to everyone. Nobody is left holding a
			// partial award that a later removal would undo from people who never got it.
			p.reverseKarma(ctx, ev.Type, team, reactor, applied, -delta, snarkLevel)
			p.restoreReaction(ctx, ev.Type, team, channel, ts, reactor, emoji, before)
			return err
		}
		if result.ShouldPersist {
			applied = append(applied, target)
		}
		if result.Message != "" {
			lines = append(lines, result.Message)
		}
	}
	if !added || len(lines) == 0 {
		return nil
	}
	// Karma is applied; a retry couldn't post the reply anyway (it would see no change).
	if err := p.webClient.PostMessage(ctx, team, channel, strings.Join(lines, "\n"), ts); err != nil {
		slog.Warn("reaction karma reply failed", "team", team, "kind", ev.Type, "channel", channel, "err", err)
	}
	return nil
}

// symbolRun turns a karma delta into the symbol run that applies it: n+1 symbols move karma by n.
func symbolRun(delta int) string {
	if delta < 0 {
		return strings.Repeat("-", -delta+1)
	}
	return strings.Repeat("+", delta+1)
}

// reverseKarma applies delta to each target to undo karma this event already gave.
// ponytail: best effort; if a reversal also fails that target keeps the karma and
// Slack's retry gives it again. Fine while ApplyDelta failures are rare and brief.
func (p *EventsProcessor) reverseKarma(ctx context.Context, kind, team, reactor string, targets []string, delta, snarkLevel int) {
	for _, target := range targets {
		if _, err := p.karmaService.HandleAction(ctx, domain.KarmaAction{
			TeamID:       team,
			ActorUserID:  reactor,
			TargetUserID: target,
			TargetHandle: "<@" + target + ">",
			SymbolRun:    symbolRun(delta),
			SnarkLevel:   snarkLevel,
			SkipWar:      true,
		}); err != nil {
			slog.Warn("could not reverse reaction karma", "team", team, "kind", kind, "err", err)
		}
	}
}

// reactionTargets is who a reaction credits: the users a typed karma message
// credited, otherwise the message's author. Nobody is credited by their own reaction.
func (p *EventsProcessor) reactionTargets(ctx context.Context, team, channel, ts, author, reactor string) ([]string, error) {
	targets, ok, err := p.reactionState.KarmaTargets(ctx, team, channel, ts)
	if err != nil {
		return nil, err
	}
	if !ok {
		return []string{author}, nil
	}
	out := make([]string, 0, len(targets))
	for _, t := range targets {
		if t != reactor {
			out = append(out, t)
		}
	}
	return out, nil
}

// restoreReaction sets emoji back to its state in before (best effort).
func (p *EventsProcessor) restoreReaction(ctx context.Context, kind, team, channel, ts, reactor, emoji string, before map[string]int) {
	var err error
	if prev, ok := before[emoji]; ok {
		_, err = p.reactionState.AddReaction(ctx, team, channel, ts, reactor, emoji, prev)
	} else {
		_, err = p.reactionState.RemoveReaction(ctx, team, channel, ts, reactor, emoji)
	}
	if err != nil {
		slog.Warn("could not restore reaction state", "team", team, "kind", kind, "channel", channel, "err", err)
	}
}

// uninstallsBot reports whether the event means the bot's token is gone: the app was
// uninstalled, or the bot's token was revoked (user-token revocations don't count).
func uninstallsBot(ev SlackEvent) bool {
	switch ev.Type {
	case "app_uninstalled":
		return true
	case "tokens_revoked":
		return len(ev.Tokens.Bot) > 0
	}
	return false
}
