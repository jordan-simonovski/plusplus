package slack

import (
	"fmt"

	"plusplus/internal/domain"
)

func settingsBlocks(currentLevel int) []map[string]interface{} {
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
		"type": "static_select",
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

	return []map[string]interface{}{
		{
			"type": "section",
			"text": map[string]interface{}{
				"type": "mrkdwn",
				"text": "*Channel settings*\n• Reply mode: `/settings reply_mode thread|channel`\n• Snark level: choose below (1 = dry, 10 = spicy).",
			},
		},
		{
			"type": "actions",
			"elements": []interface{}{selectEl},
		},
	}
}
