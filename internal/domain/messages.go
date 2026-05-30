package domain

import "fmt"

func FormatKarmaAppliedMessage(targetName string, delta int, record KarmaRecord, capped bool, maxKarmaPerAction int, snarkLevel int) string {
	verb := "lost"
	if delta > 0 {
		verb = "gained"
	}

	// The whole line is italic; bolding just the name yields bold-italic.
	line := italicize(fmt.Sprintf("%s %s %d karma. Total: %d.", bold(targetName), verb, delta, record.KarmaTotal))
	if !capped {
		return line
	}

	if delta > 0 {
		return fmt.Sprintf("%s\n%s\n%s",
			italicize(RandomCappedAwardSnark(maxKarmaPerAction, snarkLevel)),
			italicize(fmt.Sprintf("Buzzkill mode enabled: capped to %d karma.", maxKarmaPerAction)),
			line,
		)
	}

	return fmt.Sprintf("%s\n%s",
		italicize(fmt.Sprintf("Buzzkill mode enabled: capped to %d karma.", maxKarmaPerAction)),
		line,
	)
}

// FormatLeaderboardMessage renders the standings. names is aligned with entries
// (same length, same order) and holds each user's bold-italicized display name.
func FormatLeaderboardMessage(entries []KarmaRecord, names []string) string {
	if len(entries) == 0 {
		return "All-time karma leaderboard\nNo karma activity yet."
	}

	lines := "All-time karma leaderboard"
	for idx, entry := range entries {
		lines += fmt.Sprintf("\n%d. %s - %d", idx+1, boldItalic(names[idx]), entry.KarmaTotal)
	}
	return lines
}

func italicize(input string) string {
	return fmt.Sprintf("_%s_", input)
}

func bold(input string) string {
	return fmt.Sprintf("*%s*", input)
}

func boldItalic(input string) string {
	return fmt.Sprintf("*_%s_*", input)
}

// FormatGroupSelfKarmaRejection is a single line for user-group batches (plain text, no snark roulette).
func FormatGroupSelfKarmaRejection(targetName string, reason RejectionReason) string {
	switch reason {
	case RejectionSelfRemove:
		return fmt.Sprintf("%s can't remove karma from themselves.", boldItalic(targetName))
	default:
		return fmt.Sprintf("%s can't give karma to themselves.", boldItalic(targetName))
	}
}
