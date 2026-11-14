package parser

import (
	"regexp"
	"testing"
)

var slackIDPattern = regexp.MustCompile(`^[A-Z0-9]+$`)

func isKarmaDash(r rune) bool {
	switch r {
	case '-', '‐', '‑', '–', '—', '−':
		return true
	}
	return false
}

// Message text comes from any Slack user, so the parser must hold its invariants for
// arbitrary input.
func FuzzParseKarmaSegments(f *testing.F) {
	for _, seed := range []string{
		"<@U123> ++", "<@U1> -- <@U2> +++", "<!subteam^S1|@team> ++++", "<@U1> —", "<@U1> +-+",
		"++ <@U1>", "<@u1> ++", "<!subteam^S1> --", "", "<@U1>++++++++++", "−− <@U9>−−",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, text string) {
		for _, s := range ParseKarmaSegments(text) {
			if (s.UserID == "") == (s.SubteamID == "") {
				t.Fatalf("segment %+v must have exactly one of UserID and SubteamID", s)
			}
			if id := s.UserID + s.SubteamID; !slackIDPattern.MatchString(id) {
				t.Fatalf("segment %+v has a malformed ID", s)
			}
			if s.SymbolRun == "" {
				t.Fatalf("segment %+v has an empty symbol run", s)
			}
			plus, dash := true, true
			for _, r := range s.SymbolRun {
				plus = plus && r == '+'
				dash = dash && isKarmaDash(r)
			}
			if !plus && !dash {
				t.Fatalf("segment %+v mixes symbols", s)
			}
		}
	})
}
