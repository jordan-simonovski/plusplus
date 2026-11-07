package slack

import (
	"strings"
	"testing"
)

func TestSettingsBlocksAppendsVersionContext(t *testing.T) {
	blocks := settingsBlocks(5, "1.2.3", "")

	last := blocks[len(blocks)-1]
	if last["type"] != "context" {
		t.Fatalf("expected last block type context, got %v", last["type"])
	}

	elements, ok := last["elements"].([]interface{})
	if !ok || len(elements) == 0 {
		t.Fatalf("expected context elements, got %v", last["elements"])
	}

	el := elements[0].(map[string]interface{})
	if el["text"] != "PlusPlus v1.2.3" {
		t.Fatalf("unexpected version text: %v", el["text"])
	}
}

func TestSettingsBlocksKeepOversizedEmojiSummaryUnderSlackLimit(t *testing.T) {
	blocks := settingsBlocks(5, "1.2.3", "*Karma reactions*\n"+strings.Repeat(":a_very_long_custom_emoji_name: ", 200))
	for _, b := range blocks {
		if text, ok := b["text"].(map[string]interface{}); ok {
			if n := len(text["text"].(string)); n > maxSectionText {
				t.Fatalf("section text is %d chars, over Slack's %d limit", n, maxSectionText)
			}
		}
	}
}
