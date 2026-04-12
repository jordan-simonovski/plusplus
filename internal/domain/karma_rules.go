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

func EvaluateKarmaAction(input EvaluateInput) KarmaRuleOutcome {
	return EvaluateKarmaActionWithLimits(input, minSymbolCount, maxSymbolCount)
}

func EvaluateKarmaActionWithLimits(input EvaluateInput, minSymbols int, maxSymbols int) KarmaRuleOutcome {
	trimmed := strings.TrimSpace(input.SymbolRun)
	isPlus := plusOnlyPattern.MatchString(trimmed)
	isMinus := minusOnlyPattern.MatchString(trimmed)

	if !isPlus && !isMinus {
		return KarmaRuleOutcome{Kind: OutcomeReject, Reason: RejectionInvalidFormat}
	}

	if len(trimmed) < minSymbols {
		return KarmaRuleOutcome{Kind: OutcomeReject, Reason: RejectionInvalidFormat}
	}

	if input.ActorUserID == input.TargetUserID {
		reason := RejectionSelfAward
		if isMinus {
			reason = RejectionSelfRemove
		}
		return KarmaRuleOutcome{Kind: OutcomeReject, Reason: reason}
	}

	cappedCount := len(trimmed)
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
		Capped: len(trimmed) > maxSymbols,
	}
}
