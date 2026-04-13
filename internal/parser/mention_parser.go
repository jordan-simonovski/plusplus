package parser

import "regexp"

type KarmaSegmentKind int

const (
	KarmaSegmentUser KarmaSegmentKind = iota
	KarmaSegmentSubteam
)

type KarmaSegment struct {
	Kind      KarmaSegmentKind
	UserID    string // set when Kind == KarmaSegmentUser
	SubteamID string // set when Kind == KarmaSegmentSubteam (Slack user group ID, e.g. S0614TZR72F)
	SymbolRun string
}

type MentionAction struct {
	TargetUserID string
	SymbolRun    string
}

// karmaSegmentPattern: <@USER> or <!subteam^GROUP|label> followed by a + or − run.
var karmaSegmentPattern = regexp.MustCompile(
	`(?:<@([A-Z0-9]+)>|<!subteam\^([A-Z0-9]+)(?:\|[^>]+)?>)\s*(\++|[-\x{2010}\x{2011}\x{2013}\x{2014}\x{2212}]+)`,
)

func ParseMentionAction(text string) (MentionAction, bool) {
	actions := ParseMentionActions(text)
	if len(actions) == 0 {
		return MentionAction{}, false
	}
	return actions[0], true
}

// ParseMentionActions returns every user <@…> symbol-run pair (not user groups), in order.
func ParseMentionActions(text string) []MentionAction {
	segments := ParseKarmaSegments(text)
	out := make([]MentionAction, 0, len(segments))
	for _, s := range segments {
		if s.Kind == KarmaSegmentUser {
			out = append(out, MentionAction{TargetUserID: s.UserID, SymbolRun: s.SymbolRun})
		}
	}
	return out
}

// ParseKarmaSegments returns every user mention or subteam mention with its symbol run, left-to-right.
func ParseKarmaSegments(text string) []KarmaSegment {
	matches := karmaSegmentPattern.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil
	}
	out := make([]KarmaSegment, 0, len(matches))
	for _, match := range matches {
		if len(match) < 4 {
			continue
		}
		sym := match[3]
		switch {
		case match[1] != "":
			out = append(out, KarmaSegment{
				Kind:      KarmaSegmentUser,
				UserID:    match[1],
				SymbolRun: sym,
			})
		case match[2] != "":
			out = append(out, KarmaSegment{
				Kind:      KarmaSegmentSubteam,
				SubteamID: match[2],
				SymbolRun: sym,
			})
		}
	}
	return out
}
