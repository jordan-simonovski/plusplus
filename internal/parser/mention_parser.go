package parser

import "regexp"

type MentionAction struct {
	TargetUserID string
	SymbolRun    string
}

// Second group: a run of pluses, or a run of ASCII/Unicode dashes (so iOS "—" works like "--").
var mentionActionPattern = regexp.MustCompile(`<@([A-Z0-9]+)>\s*(\++|[-\x{2010}\x{2011}\x{2013}\x{2014}\x{2212}]+)`)

func ParseMentionAction(text string) (MentionAction, bool) {
	match := mentionActionPattern.FindStringSubmatch(text)
	if len(match) != 3 {
		return MentionAction{}, false
	}

	return MentionAction{
		TargetUserID: match[1],
		SymbolRun:    match[2],
	}, true
}
