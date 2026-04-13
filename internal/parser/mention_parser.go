package parser

import (
	"regexp"
	"unicode"
)

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
	idx := karmaSegmentPattern.FindAllStringSubmatchIndex(text, -1)
	if len(idx) == 0 {
		return nil
	}
	out := make([]KarmaSegment, 0, len(idx))
	for _, pair := range idx {
		if len(pair) < 8 {
			continue
		}
		symStart, symEnd := pair[6], pair[7]
		sym := text[symStart:symEnd]
		if hasPlusMinusContinuationAfter(text, symEnd) {
			continue
		}
		var userID, subteamID string
		if pair[2] != -1 && pair[3] != -1 {
			userID = text[pair[2]:pair[3]]
		}
		if pair[4] != -1 && pair[5] != -1 {
			subteamID = text[pair[4]:pair[5]]
		}
		switch {
		case userID != "":
			out = append(out, KarmaSegment{
				Kind:      KarmaSegmentUser,
				UserID:    userID,
				SymbolRun: sym,
			})
		case subteamID != "":
			out = append(out, KarmaSegment{
				Kind:      KarmaSegmentSubteam,
				SubteamID: subteamID,
				SymbolRun: sym,
			})
		}
	}
	return out
}

// hasPlusMinusContinuationAfter reports whether, after optional whitespace following a symbol run,
// there is another + or − rune. That means the karma token is mixed with further noise (e.g. "---++")
// and must not count as valid karma.
func hasPlusMinusContinuationAfter(text string, afterSym int) bool {
	for _, r := range text[afterSym:] {
		if !unicode.IsSpace(r) {
			return isPlusOrMinusKarmaRune(r)
		}
	}
	return false
}

func isPlusOrMinusKarmaRune(r rune) bool {
	if r == '+' {
		return true
	}
	switch r {
	case '-', '\u2010', '\u2011', '\u2013', '\u2014', '\u2212':
		return true
	default:
		return false
	}
}

// DedupeKarmaSegments keeps the first segment per target user or subteam; later duplicates are dropped.
func DedupeKarmaSegments(segments []KarmaSegment) []KarmaSegment {
	if len(segments) < 2 {
		return segments
	}
	seen := make(map[string]struct{}, len(segments))
	out := make([]KarmaSegment, 0, len(segments))
	for _, s := range segments {
		var key string
		switch s.Kind {
		case KarmaSegmentUser:
			key = "u:" + s.UserID
		case KarmaSegmentSubteam:
			key = "s:" + s.SubteamID
		default:
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, s)
	}
	return out
}
