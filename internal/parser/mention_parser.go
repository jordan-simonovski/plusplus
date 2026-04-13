package parser

import "regexp"

type MentionAction struct {
	TargetUserID string
	SymbolRun    string
}

// Second group: a run of pluses, or a run of ASCII/Unicode dashes (so iOS "—" works like "--").
var mentionActionPattern = regexp.MustCompile(`<@([A-Z0-9]+)>\s*(\++|[-\x{2010}\x{2011}\x{2013}\x{2014}\x{2212}]+)`)

func ParseMentionAction(text string) (MentionAction, bool) {
	actions := ParseMentionActions(text)
	if len(actions) == 0 {
		return MentionAction{}, false
	}
	return actions[0], true
}

// ParseMentionActions returns every <@USER> symbol-run pair in left-to-right order.
func ParseMentionActions(text string) []MentionAction {
	matches := mentionActionPattern.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil
	}
	out := make([]MentionAction, 0, len(matches))
	for _, match := range matches {
		if len(match) != 3 {
			continue
		}
		out = append(out, MentionAction{
			TargetUserID: match[1],
			SymbolRun:    match[2],
		})
	}
	return out
}
