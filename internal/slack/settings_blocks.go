package slack

import (
	"fmt"

	"plusplus/internal/domain"
)

// settingsBlocks renders the interactive settings panel. emojiSummary (the channel's
// karma reactions, from ReactionSettingsService.Describe) is shown when non-empty.
func settingsBlocks(currentLevel int, appVersion string, emojiSummary string) []map[string]interface{} {
	if currentLevel < domain.MinSnarkLevel || currentLevel > domain.MaxSnarkLevel {
		currentLevel = domain.DefaultSnarkLevel
	}

	options := make([]map[string]interface{}, 0, domain.MaxSnarkLevel)
	for i := 1; i <= domain.MaxSnarkLevel; i++ {
		label := fmt.Sprintf("%d", i)
		if i == domain.DefaultSnarkLevel {
			label = fmt.Sprintf("%d — default", i)
		}
		options = append(options, map[string]interface{}{
			"text": map[string]interface{}{
				"type": "plain_text",
				"text": label,
			},
			"value": fmt.Sprintf("%d", i),
		})
	}

	var initialOption map[string]interface{}
	want := fmt.Sprintf("%d", currentLevel)
	for _, opt := range options {
		if opt["value"] == want {
			initialOption = opt
			break
		}
	}

	selectEl := map[string]interface{}{
		"type":      "static_select",
		"action_id": "snark_level_select",
		"placeholder": map[string]interface{}{
			"type": "plain_text",
			"text": "Snark level",
		},
		"options": options,
	}
	if initialOption != nil {
		selectEl["initial_option"] = initialOption
	}

	blocks := []map[string]interface{}{
		{
			"type": "section",
			"text": map[string]interface{}{
				"type": "mrkdwn",
				"text": "*Channel settings*\n• Reply mode: add `reply_mode thread|channel` to this command\n• Snark level: choose below (1 = dry, 10 = spicy).",
			},
		},
		{
			"type":     "actions",
			"elements": []interface{}{selectEl},
		},
	}
	if emojiSummary != "" {
		blocks = append(blocks,
			map[string]interface{}{"type": "divider"},
			map[string]interface{}{
				"type": "section",
				"text": map[string]interface{}{"type": "mrkdwn", "text": emojiSummary},
			},
		)
	}
	return append(blocks,
		map[string]interface{}{
			"type": "context",
			"elements": []interface{}{
				map[string]interface{}{
					"type": "mrkdwn",
					"text": fmt.Sprintf("PlusPlus v%s", appVersion),
				},
			},
		},
	)
}
