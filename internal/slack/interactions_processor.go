package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"plusplus/internal/domain"
)

type SettingsInteractionService interface {
	GetChannelSettings(ctx context.Context, teamID string, channelID string) (ReplyMode, int, error)
	SetReplyMode(ctx context.Context, teamID string, channelID string, actorUserID string, mode ReplyMode) (string, error)
	SetSnarkLevel(ctx context.Context, teamID string, channelID string, actorUserID string, level int) (string, error)
}

type InteractionsProcessor struct {
	settings   SettingsInteractionService
	reactions  ReactionSettingsCommands
	version    string
	httpClient *http.Client
}

func NewInteractionsProcessor(settings SettingsInteractionService, version string) *InteractionsProcessor {
	return &InteractionsProcessor{settings: settings, version: version, httpClient: newHTTPClient()}
}

// WithReactionSettings shows the channel's karma reactions on the panel and lets people
// change them from it.
func (p *InteractionsProcessor) WithReactionSettings(reactions ReactionSettingsCommands) *InteractionsProcessor {
	p.reactions = reactions
	return p
}

// HandleInteraction handles one interactive-component payload. body has already passed
// VerifiedBody. A change on the settings panel is saved, then the panel is redrawn in
// place through response_url: Slack ignores this response's body for block_actions.
func (p *InteractionsProcessor) HandleInteraction(w http.ResponseWriter, r *http.Request, body []byte) {
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

	ctx, team, channel, user := r.Context(), payload.Team.ID, payload.Channel.ID, payload.User.ID
	var notice string
	for _, action := range payload.Actions {
		value := action.SelectedOption.Value
		switch action.ActionID {
		case actionSnarkLevel:
			level, err := strconv.Atoi(value)
			if err != nil || level < domain.MinSnarkLevel || level > domain.MaxSnarkLevel {
				notice = domain.PanelBadSnarkLevel
				continue
			}
			if notice, err = p.settings.SetSnarkLevel(ctx, team, channel, user, level); err != nil {
				slog.Error("update snark level failed", "team", team, "kind", "interaction", "err", err)
				http.Error(w, "failed to update settings", http.StatusInternalServerError)
				return
			}
		case actionReplyMode:
			mode := ReplyMode(value)
			if mode != ReplyModeThread && mode != ReplyModeChannel {
				notice = domain.PanelBadReplyMode
				continue
			}
			if notice, err = p.settings.SetReplyMode(ctx, team, channel, user, mode); err != nil {
				slog.Error("update reply mode failed", "team", team, "kind", "interaction", "err", err)
				http.Error(w, "failed to update settings", http.StatusInternalServerError)
				return
			}
		case actionEmojiInput, actionEmojiReset:
			if p.reactions == nil {
				continue
			}
			var err error
			if action.ActionID == actionEmojiReset {
				// Always this channel: a workspace box left ticked from an earlier edit must not
				// turn Reset into a workspace wipe. `/plusplus-settings emoji workspace reset` does that.
				notice, err = p.reactions.Reset(ctx, team, channel, false, user)
			} else if tier := payload.State.selected(actionEmojiTier); tier == "" {
				notice = domain.PanelPickTierFirst
			} else {
				workspace := payload.State.selected(actionEmojiScope) == emojiScopeWorkspace
				notice, err = p.reactions.SetTier(ctx, team, channel, workspace, user, tier, strings.Fields(action.Value))
			}
			var input *InputError
			if errors.As(err, &input) {
				notice = domain.FormatPanelInputError(input.Msg)
			} else if err != nil {
				slog.Error("update karma reactions failed", "team", team, "kind", "interaction", "err", err)
				http.Error(w, "failed to update settings", http.StatusInternalServerError)
				return
			}
		}
	}
	if notice == "" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// The setting is saved; a panel that can't be redrawn is cosmetic, so Slack still gets 200.
	panel, err := loadSettingsPanel(ctx, p.settings, p.reactions, team, channel, "interaction", p.version)
	if err != nil {
		slog.Warn("settings panel refresh failed", "team", team, "kind", "interaction", "err", err)
		w.WriteHeader(http.StatusOK)
		return
	}
	panel.notice = notice
	// Slack wants the ack within 3 seconds, so the redraw gets what's left of that, roughly.
	redrawCtx, cancel := context.WithTimeout(ctx, redrawTimeout)
	defer cancel()
	if err := p.replacePanel(redrawCtx, payload.ResponseURL, notice, settingsBlocks(panel)); err != nil {
		slog.Warn("settings panel refresh failed", "team", team, "kind", "interaction", "err", err)
	}
	w.WriteHeader(http.StatusOK)
}

// redrawTimeout bounds the panel redraw so the interaction is still acked in time.
const redrawTimeout = 2 * time.Second

// slackHookHosts are where Slack's response_urls point (commercial and GovSlack).
var slackHookHosts = map[string]bool{"hooks.slack.com": true, "hooks.slack-gov.com": true}

// replacePanel swaps the ephemeral panel for blocks. response_url comes from a verified
// payload, but it is still checked to be Slack's, so the bot never posts anywhere else.
func (p *InteractionsProcessor) replacePanel(ctx context.Context, responseURL, text string, blocks []map[string]interface{}) error {
	u, err := url.Parse(responseURL)
	if err != nil || u.Scheme != "https" || !slackHookHosts[u.Host] {
		return fmt.Errorf("unexpected response_url host")
	}
	body, err := json.Marshal(map[string]interface{}{"replace_original": true, "text": text, "blocks": blocks})
	if err != nil {
		return fmt.Errorf("encode panel: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build panel request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.httpClient.Do(req)
	if err != nil {
		// A *url.Error carries the response_url, which works like a short-lived credential:
		// keep only the cause so it never reaches the logs.
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		return fmt.Errorf("post panel: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("post panel: status %d", resp.StatusCode)
	}
	return nil
}

type slackInteractionPayload struct {
	Type        string             `json:"type"`
	User        slackEntityID      `json:"user"`
	Team        slackEntityID      `json:"team"`
	Channel     slackEntityID      `json:"channel"`
	ResponseURL string             `json:"response_url"`
	Actions     []slackBlockAction `json:"actions"`
	State       slackMessageState  `json:"state"`
}

// slackMessageState is what every input on the message holds when an action fires,
// keyed by block ID (generated by Slack) and then action ID.
type slackMessageState struct {
	Values map[string]map[string]struct {
		SelectedOption  *slackOption  `json:"selected_option"`
		SelectedOptions []slackOption `json:"selected_options"`
	} `json:"values"`
}

type slackOption struct {
	Value string `json:"value"`
}

// selected returns the chosen value of the select or checkbox with actionID, or "".
func (s slackMessageState) selected(actionID string) string {
	for _, block := range s.Values {
		if v, ok := block[actionID]; ok {
			if v.SelectedOption != nil {
				return v.SelectedOption.Value
			}
			if len(v.SelectedOptions) > 0 {
				return v.SelectedOptions[0].Value
			}
		}
	}
	return ""
}

type slackEntityID struct {
	ID string `json:"id"`
}

type slackBlockAction struct {
	ActionID       string `json:"action_id"`
	Value          string `json:"value"` // a text input's contents
	SelectedOption struct {
		Value string `json:"value"`
	} `json:"selected_option"`
}
