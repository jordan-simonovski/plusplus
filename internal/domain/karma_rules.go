package domain

import (
	"regexp"
	"strings"
)

const (
	minSymbolCount = 2
	maxSymbolCount = 6
)

var (
	plusOnlyPattern  = regexp.MustCompile(`^\++$`)
	minusOnlyPattern = regexp.MustCompile(`^\-+$`)
)

// isMinusRune returns true for ASCII hyphen-minus and common Unicode dashes/minuses
// that may appear when clients substitute "--" (e.g. iOS typographic em/en dash).
func isMinusRune(r rune) bool {
	switch r {
	case '-', '\u2010', '\u2011', '\u2013', '\u2014', '\u2212':
		return true
	default:
		return false
	}
}

func isUniformMinusRun(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !isMinusRune(r) {
			return false
		}
	}
	return true
}

// expandMinusRunToHyphens maps each em dash or en dash to two ASCII hyphens so a
// single "—" or "–" counts the same as "--" for karma removal.
func expandMinusRunToHyphens(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\u2013', '\u2014': // en dash, em dash
			b.WriteString("--")
		case '-', '\u2010', '\u2011', '\u2212':
			b.WriteByte('-')
		default:
			return ""
		}
	}
	return b.String()
}

func EvaluateKarmaAction(input EvaluateInput) KarmaRuleOutcome {
	return EvaluateKarmaActionWithLimits(input, minSymbolCount, maxSymbolCount)
}

func EvaluateKarmaActionWithLimits(input EvaluateInput, minSymbols int, maxSymbols int) KarmaRuleOutcome {
	trimmed := strings.TrimSpace(input.SymbolRun)
	isPlus := plusOnlyPattern.MatchString(trimmed)
	isMinus := false
	var minusExpanded string
	if isUniformMinusRun(trimmed) {
		minusExpanded = expandMinusRunToHyphens(trimmed)
		isMinus = minusOnlyPattern.MatchString(minusExpanded)
	}

	if !isPlus && !isMinus {
		return KarmaRuleOutcome{Kind: OutcomeReject, Reason: RejectionInvalidFormat}
	}

	effectiveRun := trimmed
	if isMinus {
		effectiveRun = minusExpanded
	}

	if len(effectiveRun) < minSymbols {
		return KarmaRuleOutcome{Kind: OutcomeReject, Reason: RejectionInvalidFormat}
	}

	if input.ActorUserID == input.TargetUserID {
		reason := RejectionSelfAward
		if isMinus {
			reason = RejectionSelfRemove
		}
		return KarmaRuleOutcome{Kind: OutcomeReject, Reason: reason}
	}

	cappedCount := len(effectiveRun)
	if cappedCount > maxSymbols {
		cappedCount = maxSymbols
	}
	points := cappedCount - 1
	if isMinus {
		points = points * -1
	}

	return KarmaRuleOutcome{
		Kind:   OutcomeApply,
		Delta:  points,
		Capped: len(effectiveRun) > maxSymbols,
	}
}
