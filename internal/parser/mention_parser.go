package parser

import "regexp"

type MentionAction struct {
	TargetUserID string
	SymbolRun    string
}

var mentionActionPattern = regexp.MustCompile(`<@([A-Z0-9]+)>\s*([+-]+)`)

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
