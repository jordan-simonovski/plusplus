package domain

import "testing"

func TestEvaluateKarmaAction(t *testing.T) {
	tests := []struct {
		name     string
		input    EvaluateInput
		expected KarmaRuleOutcome
	}{
		{
			name: "rejects mixed symbol input",
			input: EvaluateInput{
				ActorUserID:  "A",
				TargetUserID: "B",
				SymbolRun:    "+-+",
			},
			expected: KarmaRuleOutcome{Kind: OutcomeReject, Reason: RejectionInvalidFormat},
		},
		{
			name: "rejects too short input",
			input: EvaluateInput{
				ActorUserID:  "A",
				TargetUserID: "B",
				SymbolRun:    "+",
			},
			expected: KarmaRuleOutcome{Kind: OutcomeReject, Reason: RejectionInvalidFormat},
		},
		{
			name: "rejects self award",
			input: EvaluateInput{
				ActorUserID:  "A",
				TargetUserID: "A",
				SymbolRun:    "+++",
			},
			expected: KarmaRuleOutcome{Kind: OutcomeReject, Reason: RejectionSelfAward},
		},
		{
			name: "rejects self remove",
			input: EvaluateInput{
				ActorUserID:  "A",
				TargetUserID: "A",
				SymbolRun:    "---",
			},
			expected: KarmaRuleOutcome{Kind: OutcomeReject, Reason: RejectionSelfRemove},
		},
		{
			name: "applies plus run",
			input: EvaluateInput{
				ActorUserID:  "A",
				TargetUserID: "B",
				SymbolRun:    "++++",
			},
			expected: KarmaRuleOutcome{Kind: OutcomeApply, Delta: 3, Capped: false},
		},
		{
			name: "applies minus run",
			input: EvaluateInput{
				ActorUserID:  "A",
				TargetUserID: "B",
				SymbolRun:    "----",
			},
			expected: KarmaRuleOutcome{Kind: OutcomeApply, Delta: -3, Capped: false},
		},
		{
			name: "em dash counts as two hyphens for removal",
			input: EvaluateInput{
				ActorUserID:  "A",
				TargetUserID: "B",
				SymbolRun:    "\u2014", // —
			},
			expected: KarmaRuleOutcome{Kind: OutcomeApply, Delta: -1, Capped: false},
		},
		{
			name: "two em dashes match four hyphens",
			input: EvaluateInput{
				ActorUserID:  "A",
				TargetUserID: "B",
				SymbolRun:    "\u2014\u2014",
			},
			expected: KarmaRuleOutcome{Kind: OutcomeApply, Delta: -3, Capped: false},
		},
		{
			name: "caps oversized plus run",
			input: EvaluateInput{
				ActorUserID:  "A",
				TargetUserID: "B",
				SymbolRun:    "+++++++",
			},
			expected: KarmaRuleOutcome{Kind: OutcomeApply, Delta: 5, Capped: true},
		},
		{
			name: "trims whitespace before evaluating",
			input: EvaluateInput{
				ActorUserID:  "A",
				TargetUserID: "B",
				SymbolRun:    "  ++  ",
			},
			expected: KarmaRuleOutcome{Kind: OutcomeApply, Delta: 1, Capped: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluateKarmaAction(tt.input)
			if got != tt.expected {
				t.Fatalf("unexpected outcome: got %+v want %+v", got, tt.expected)
			}
		})
	}
}
