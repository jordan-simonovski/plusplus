package parser

import "testing"

func TestParseMentionAction(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantUserID string
		wantRun    string
		ok         bool
	}{
		{
			name:       "parses mention with pluses",
			input:      "<@UBOT> <@U123> +++++",
			wantUserID: "U123",
			wantRun:    "+++++",
			ok:         true,
		},
		{
			name:       "parses mention with minuses",
			input:      "hey <@U555> ---",
			wantUserID: "U555",
			wantRun:    "---",
			ok:         true,
		},
		{
			name:       "parses mention with em dash",
			input:      "hey <@U777> \u2014",
			wantUserID: "U777",
			wantRun:    "\u2014",
			ok:         true,
		},
		{
			name:  "rejects missing symbol run",
			input: "<@U123> hello",
			ok:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseMentionAction(tt.input)
			if ok != tt.ok {
				t.Fatalf("unexpected parse status: got %v want %v", ok, tt.ok)
			}
			if !tt.ok {
				return
			}
			if got.TargetUserID != tt.wantUserID || got.SymbolRun != tt.wantRun {
				t.Fatalf("unexpected parse result: got %+v", got)
			}
		})
	}
}
