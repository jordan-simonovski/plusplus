package domain

import "fmt"

func FormatKarmaAppliedMessage(targetHandle string, delta int, record KarmaRecord, capped bool, maxKarmaPerAction int) string {
	verb := "lost"
	if delta > 0 {
		verb = "gained"
	}

	line := fmt.Sprintf("%s %s %d karma. Total: %d. Max: %d.", targetHandle, verb, delta, record.KarmaTotal, record.KarmaMax)
	if !capped {
		return line
	}

	if delta > 0 {
		return fmt.Sprintf("%s\nBuzzkill mode enabled: capped to %d karma.\n%s", RandomCappedAwardSnark(maxKarmaPerAction), maxKarmaPerAction, line)
	}

	return fmt.Sprintf("Buzzkill mode enabled: capped to %d karma.\n%s", maxKarmaPerAction, line)
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
