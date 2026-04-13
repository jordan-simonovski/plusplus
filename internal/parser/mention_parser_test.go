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

	multiTests := []struct {
		name    string
		input   string
		want    []MentionAction
		wantLen int
	}{
		{
			name:    "two recipients in one message",
			input:   "Bravo <@U1> ++++++ , from <@U2> +++++",
			wantLen: 2,
			want: []MentionAction{
				{TargetUserID: "U1", SymbolRun: "++++++"},
				{TargetUserID: "U2", SymbolRun: "+++++"},
			},
		},
		{
			name:    "four recipients mixed plus and minus",
			input:   "<@U1> +++ <@U2> +++++ <@U3> ++++ <@U4> ----",
			wantLen: 4,
			want: []MentionAction{
				{TargetUserID: "U1", SymbolRun: "+++"},
				{TargetUserID: "U2", SymbolRun: "+++++"},
				{TargetUserID: "U3", SymbolRun: "++++"},
				{TargetUserID: "U4", SymbolRun: "----"},
			},
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

	for _, tt := range multiTests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseMentionActions(tt.input)
			if len(got) != tt.wantLen {
				t.Fatalf("unexpected length: got %d want %d", len(got), tt.wantLen)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Fatalf("unexpected parse result at %d: got %+v want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}
