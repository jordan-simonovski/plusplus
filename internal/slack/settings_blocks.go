package slack

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	"plusplus/internal/domain"
)

// maxSectionText is Slack's limit for a section block's text; one over it makes Slack
// reject the whole message, so the panel would fail to open.
const maxSectionText = 3000

// Action IDs on the settings panel, handled by InteractionsProcessor.
const (
	actionReplyMode     = "reply_mode_radio"
	actionSnarkLevel    = "snark_level_select"
	actionEmojiTier     = "emoji_tier_select"  // read from state when emojis are submitted
	actionEmojiScope    = "emoji_scope_check"  // read from state when emojis are submitted
	actionEmojiInput    = "emoji_tier_input"   // Enter saves the typed emojis to the picked tier
	actionEmojiReset    = "emoji_reset_button" // drops this channel's (or the workspace's) emojis
	emojiScopeWorkspace = "workspace"
)

// emojiDescriber summarises a channel's karma reactions for the panel.
type emojiDescriber interface {
	Describe(ctx context.Context, teamID, channelID string) (string, error)
}

// settingsPanel is what the settings panel shows.
type settingsPanel struct {
	channelID    string
	mode         ReplyMode
	snark        int
	emojiSummary string // the channel's karma reactions; omitted when empty
	canEditEmoji bool   // show the controls for changing karma reactions
	notice       string // what just changed; omitted when empty
	version      string
}

// loadSettingsPanel reads a channel's current settings for the panel. The emoji summary
// is extra context: if it can't load, the panel still opens without it.
func loadSettingsPanel(ctx context.Context, settings ChannelSettingsProvider, reactions emojiDescriber, teamID, channelID, kind, version string) (settingsPanel, error) {
	mode, snark, err := settings.GetChannelSettings(ctx, teamID, channelID)
	if err != nil {
		return settingsPanel{}, err
	}
	panel := settingsPanel{channelID: channelID, mode: mode, snark: snark, version: version, canEditEmoji: reactions != nil}
	if reactions != nil {
		if summary, err := reactions.Describe(ctx, teamID, channelID); err == nil {
			panel.emojiSummary = summary
		} else {
			slog.Warn("emoji summary unavailable for settings panel", "team", teamID, "kind", kind, "err", err)
		}
	}
	return panel, nil
}

// settingsBlocks renders the interactive settings panel.
func settingsBlocks(p settingsPanel) []map[string]interface{} {
	snark := p.snark
	if snark < domain.MinSnarkLevel || snark > domain.MaxSnarkLevel {
		snark = domain.DefaultSnarkLevel
	}
	snarkOptions := make([]map[string]interface{}, 0, domain.MaxSnarkLevel)
	for i := domain.MinSnarkLevel; i <= domain.MaxSnarkLevel; i++ {
		label := fmt.Sprintf("%d", i)
		switch i {
		case domain.MinSnarkLevel:
			label += " (dry)"
		case domain.DefaultSnarkLevel:
			label += " (default)"
		case domain.MaxSnarkLevel:
			label += " (spicy)"
		}
		snarkOptions = append(snarkOptions, option(label, fmt.Sprintf("%d", i)))
	}

	modeOptions := []map[string]interface{}{
		option("In a thread", string(ReplyModeThread)),
		option("In the channel", string(ReplyModeChannel)),
	}
	modeInitial := modeOptions[0]
	if p.mode == ReplyModeChannel {
		modeInitial = modeOptions[1]
	}

	title := "*Channel settings*"
	if p.channelID != "" {
		title = fmt.Sprintf("*Settings for <#%s>*", p.channelID)
	}
	blocks := []map[string]interface{}{
		mrkdwnSection(title),
		{
			"type": "section",
			"text": mrkdwn("*Karma replies*\nWhere PlusPlus answers a karma message."),
			"accessory": map[string]interface{}{
				"type":           "radio_buttons",
				"action_id":      actionReplyMode,
				"options":        modeOptions,
				"initial_option": modeInitial,
			},
		},
		{
			"type": "section",
			"text": mrkdwn("*Snark*\nHow sassy the replies are."),
			"accessory": map[string]interface{}{
				"type":           "static_select",
				"action_id":      actionSnarkLevel,
				"placeholder":    plainText("Snark level"),
				"options":        snarkOptions,
				"initial_option": snarkOptions[snark-domain.MinSnarkLevel],
			},
		},
	}

	summary := p.emojiSummary
	if len(summary) > maxSectionText {
		summary = "*Karma reactions*\nThis channel has a lot of karma emojis. Run `/plusplus-settings emoji` to see them all."
	}
	if summary != "" {
		blocks = append(blocks, map[string]interface{}{"type": "divider"}, mrkdwnSection(summary))
	}
	if p.canEditEmoji {
		blocks = append(blocks, emojiEditBlocks()...)
	}

	footer := fmt.Sprintf("PlusPlus v%s", p.version)
	if p.notice != "" {
		footer = p.notice + "  ·  " + footer
	}
	return append(blocks, map[string]interface{}{
		"type":     "context",
		"elements": []interface{}{mrkdwn(footer)},
	})
}

// maxEmojiInput caps the emoji field: room for the 10 emojis a tier allows, and short
// enough that an echoed mistake keeps the redrawn panel under Slack's size limits.
const maxEmojiInput = 500

// emojiEditBlocks lets people point a tier at new emojis without typing a command. The
// tier and scope are read from the message state when Enter is pressed in the input.
func emojiEditBlocks() []map[string]interface{} {
	var tiers []map[string]interface{}
	for t := domain.MaxReactionTier; t >= -domain.MaxReactionTier; t-- {
		switch {
		case t > 0:
			tiers = append(tiers, option(fmt.Sprintf("Gives %d", t), strconv.Itoa(t)))
		case t < 0:
			tiers = append(tiers, option(fmt.Sprintf("Takes %d", -t), strconv.Itoa(t)))
		}
	}
	return []map[string]interface{}{
		mrkdwnSection("*Change a reaction*"),
		{
			"type": "actions",
			"elements": []interface{}{
				map[string]interface{}{
					"type":        "static_select",
					"action_id":   actionEmojiTier,
					"placeholder": plainText("Pick a tier"),
					"options":     tiers,
				},
				map[string]interface{}{
					"type":      "checkboxes",
					"action_id": actionEmojiScope,
					"options":   []interface{}{option("Whole workspace (admins)", emojiScopeWorkspace)},
				},
				map[string]interface{}{
					"type":      "button",
					"action_id": actionEmojiReset,
					"text":      plainText("Reset"),
					"confirm": map[string]interface{}{
						"title":   plainText("Reset karma reactions?"),
						"text":    plainText("This channel goes back to the workspace's karma reactions."),
						"confirm": plainText("Reset"),
						"deny":    plainText("Cancel"),
					},
				},
			},
		},
		{
			"type":            "input",
			"dispatch_action": true,
			"label":           plainText("New emojis for that tier"),
			"hint":            plainText("Like :tada: :rocket:, then press Enter. They replace the tier's current emojis."),
			"element": map[string]interface{}{
				"type":                   "plain_text_input",
				"action_id":              actionEmojiInput,
				"placeholder":            plainText(":tada: :rocket:"),
				"max_length":             maxEmojiInput,
				"dispatch_action_config": map[string]interface{}{"trigger_actions_on": []string{"on_enter_pressed"}},
			},
		},
	}
}

func option(label, value string) map[string]interface{} {
	return map[string]interface{}{"text": plainText(label), "value": value}
}

func plainText(text string) map[string]interface{} {
	return map[string]interface{}{"type": "plain_text", "text": text}
}

func mrkdwn(text string) map[string]interface{} {
	return map[string]interface{}{"type": "mrkdwn", "text": text}
}

func mrkdwnSection(text string) map[string]interface{} {
	return map[string]interface{}{"type": "section", "text": mrkdwn(text)}
}
