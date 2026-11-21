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

// mentionPattern matches <@USER> or <!subteam^GROUP|label>.
const mentionPattern = `(?:<@([A-Z0-9]+)>|<!subteam\^([A-Z0-9]+)(?:\|[^>]+)?>)`

var mentionRegexp = regexp.MustCompile(mentionPattern)

// karmaSegmentPattern: one or more mentions separated only by whitespace, then a + or −
// run they all share, so "<@A> <@B> ++" credits both.
var karmaSegmentPattern = regexp.MustCompile(
	`((?:` + mentionPattern + `\s*)+)(\++|[-\x{2010}\x{2011}\x{2013}\x{2014}\x{2212}]+)`,
)

// ParseKarmaSegments returns every user mention or subteam mention with its symbol run, left-to-right.
func ParseKarmaSegments(text string) []KarmaSegment {
	var out []KarmaSegment
	for _, m := range karmaSegmentPattern.FindAllStringSubmatchIndex(text, -1) {
		mentions, sym := text[m[2]:m[3]], text[m[len(m)-2]:m[len(m)-1]]
		if hasPlusMinusContinuationAfter(text, m[len(m)-1]) {
			continue
		}
		for _, sub := range mentionRegexp.FindAllStringSubmatch(mentions, -1) {
			if sub[1] != "" {
				out = append(out, KarmaSegment{Kind: KarmaSegmentUser, UserID: sub[1], SymbolRun: sym})
			} else {
				out = append(out, KarmaSegment{Kind: KarmaSegmentSubteam, SubteamID: sub[2], SymbolRun: sym})
			}
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
