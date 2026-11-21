package slack

import (
	"strings"
	"testing"
)

func TestSettingsBlocksAppendsVersionContext(t *testing.T) {
	blocks := settingsBlocks(settingsPanel{snark: 5, version: "1.2.3"})

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
	blocks := settingsBlocks(settingsPanel{snark: 5, version: "1.2.3", emojiSummary: "*Karma reactions*\n" + strings.Repeat(":a_very_long_custom_emoji_name: ", 200)})
	for _, b := range blocks {
		if text, ok := b["text"].(map[string]interface{}); ok {
			if n := len(text["text"].(string)); n > maxSectionText {
				t.Fatalf("section text is %d chars, over Slack's %d limit", n, maxSectionText)
			}
		}
	}
}

func TestSettingsBlocksShowCurrentValues(t *testing.T) {
	for _, tc := range []struct {
		panel     settingsPanel
		wantMode  string
		wantSnark string
	}{
		{settingsPanel{mode: ReplyModeChannel, snark: 9}, "channel", "9"},
		{settingsPanel{mode: ReplyModeThread, snark: 0}, "thread", "5"}, // out of range: the default
	} {
		var mode, snark string
		for _, b := range settingsBlocks(tc.panel) {
			acc, ok := b["accessory"].(map[string]interface{})
			if !ok {
				continue
			}
			value := acc["initial_option"].(map[string]interface{})["value"].(string)
			switch acc["action_id"] {
			case actionReplyMode:
				mode = value
			case actionSnarkLevel:
				snark = value
			}
		}
		if mode != tc.wantMode || snark != tc.wantSnark {
			t.Fatalf("panel shows mode %q snark %q, want %q %q", mode, snark, tc.wantMode, tc.wantSnark)
		}
	}
}

func TestSettingsBlocksEmojiControlsOnlyWhenEditable(t *testing.T) {
	has := func(p settingsPanel) bool {
		for _, b := range settingsBlocks(p) {
			if b["type"] == "input" && b["dispatch_action"] == true {
				return true
			}
		}
		return false
	}
	if !has(settingsPanel{canEditEmoji: true}) {
		t.Fatal("an editable panel should have the emoji input")
	}
	if has(settingsPanel{}) {
		t.Fatal("a panel without reaction settings shouldn't offer emoji editing")
	}
}
