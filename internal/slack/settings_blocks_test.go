package slack

import "testing"

func TestSettingsBlocksAppendsVersionContext(t *testing.T) {
	blocks := settingsBlocks(5, "1.2.3")

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
