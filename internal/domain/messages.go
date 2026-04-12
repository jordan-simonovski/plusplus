package domain

import "fmt"

func FormatKarmaAppliedMessage(targetHandle string, delta int, record KarmaRecord, capped bool, maxKarmaPerAction int) string {
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
			italicize(RandomCappedAwardSnark(maxKarmaPerAction)),
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
