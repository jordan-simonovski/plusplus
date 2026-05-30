package domain

import (
	"strings"
	"testing"
)

func TestFormatKarmaAppliedMessageIncludesSnarkForCappedAward(t *testing.T) {
	message := FormatKarmaAppliedMessage("<@U2>", 5, KarmaRecord{
		KarmaTotal: 42,
		KarmaMax:   42,
	}, true, 5, DefaultSnarkLevel)

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
	}, true, 5, DefaultSnarkLevel)

	for _, sub := range []string{
		"Nobody gets more than",
		"forklift",
		"Overclock denied",
	} {
		if strings.Contains(message, sub) {
			t.Fatalf("did not expect capped award snark in removal message: %q", message)
		}
	}
}

func TestFormatKarmaAppliedMessageIncludesTotalWithoutMax(t *testing.T) {
	message := FormatKarmaAppliedMessage("<@U2>", 3, KarmaRecord{
		KarmaTotal: 12,
		KarmaMax:   42,
	}, false, 5, DefaultSnarkLevel)

	if !strings.Contains(message, "Total: 12.") {
		t.Fatalf("expected total in message: %q", message)
	}
	if strings.Contains(message, "Max:") {
		t.Fatalf("did not expect max in message: %q", message)
	}
	if message != "_*<@U2>* gained 3 karma. Total: 12._" {
		t.Fatalf("unexpected message: %q", message)
	}
}

func TestFormatKarmaAppliedMessageBoldItalicizesName(t *testing.T) {
	message := FormatKarmaAppliedMessage("Jane Doe", 2, KarmaRecord{KarmaTotal: 5}, false, 5, DefaultSnarkLevel)
	if message != "_*Jane Doe* gained 2 karma. Total: 5._" {
		t.Fatalf("unexpected message: %q", message)
	}
}

func TestFormatGroupSelfKarmaRejection(t *testing.T) {
	if got := FormatGroupSelfKarmaRejection("Jane", RejectionSelfAward); got != "*_Jane_* can't give karma to themselves." {
		t.Fatalf("award: %q", got)
	}
	if got := FormatGroupSelfKarmaRejection("Jane", RejectionSelfRemove); got != "*_Jane_* can't remove karma from themselves." {
		t.Fatalf("remove: %q", got)
	}
}

func TestFormatLeaderboardMessageBoldItalicizesNames(t *testing.T) {
	entries := []KarmaRecord{{UserID: "U9", KarmaTotal: 14}, {UserID: "U7", KarmaTotal: 12}}
	names := []string{"Jane", "John"}
	want := "All-time karma leaderboard\n1. *_Jane_* - 14\n2. *_John_* - 12"
	if got := FormatLeaderboardMessage(entries, names); got != want {
		t.Fatalf("unexpected leaderboard: %q", got)
	}
}
