package domain

import (
	"strings"
	"testing"
)

func TestFormatKarmaAppliedMessageIncludesSnarkForCappedAward(t *testing.T) {
	message := FormatKarmaAppliedMessage("<@U2>", 5, KarmaRecord{
		KarmaTotal: 42,
		KarmaMax:   42,
	}, true, 5)

	if !strings.Contains(message, "_Buzzkill mode enabled: capped to 5 karma._") {
		t.Fatalf("expected buzzkill line in message: %q", message)
	}

	lines := strings.Split(message, "\n")
	if len(lines) < 3 {
		t.Fatalf("expected multi-line capped message, got: %q", message)
	}
	if lines[0] == "" {
		t.Fatalf("expected non-empty snark line in message: %q", message)
	}
	if lines[0] == "_Buzzkill mode enabled: capped to 5 karma._" {
		t.Fatalf("expected randomized snark prefix, got buzzkill line first: %q", message)
	}
}

func TestFormatKarmaAppliedMessageDoesNotIncludeAwardSnarkForCappedRemoval(t *testing.T) {
	message := FormatKarmaAppliedMessage("<@U2>", -5, KarmaRecord{
		KarmaTotal: 12,
		KarmaMax:   42,
	}, true, 5)

	for _, snark := range cappedAwardSnarkMessages {
		if strings.Contains(message, snark) {
			t.Fatalf("did not expect capped award snark in removal message: %q", message)
		}
	}
}

func TestFormatKarmaAppliedMessageIncludesTotalWithoutMax(t *testing.T) {
	message := FormatKarmaAppliedMessage("<@U2>", 3, KarmaRecord{
		KarmaTotal: 12,
		KarmaMax:   42,
	}, false, 5)

	if !strings.Contains(message, "Total: 12.") {
		t.Fatalf("expected total in message: %q", message)
	}
	if strings.Contains(message, "Max:") {
		t.Fatalf("did not expect max in message: %q", message)
	}
	if message != "_<@U2> gained 3 karma. Total: 12._" {
		t.Fatalf("unexpected message: %q", message)
	}
}
