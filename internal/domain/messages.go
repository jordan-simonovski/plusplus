package domain

import "fmt"

func FormatKarmaAppliedMessage(targetHandle string, delta int, record KarmaRecord, capped bool, maxKarmaPerAction int, snarkLevel int) string {
	verb := "lost"
	if delta > 0 {
		verb = "gained"
	}

	line := italicize(fmt.Sprintf("%s %s %d karma. Total: %d.", targetHandle, verb, delta, record.KarmaTotal))
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

func FormatLeaderboardMessage(entries []KarmaRecord) string {
	if len(entries) == 0 {
		return "All-time karma leaderboard\nNo karma activity yet."
	}

	lines := "All-time karma leaderboard"
	for idx, entry := range entries {
		lines += fmt.Sprintf("\n%d. <@%s> - %d", idx+1, entry.UserID, entry.KarmaTotal)
	}
	return lines
}

func italicize(input string) string {
	return fmt.Sprintf("_%s_", input)
}

// FormatGroupSelfKarmaRejection is a single line for user-group batches (plain text, no snark roulette).
func FormatGroupSelfKarmaRejection(targetHandle string, reason RejectionReason) string {
	switch reason {
	case RejectionSelfRemove:
		return fmt.Sprintf("%s can't remove karma from themselves.", targetHandle)
	default:
		return fmt.Sprintf("%s can't give karma to themselves.", targetHandle)
	}
}
