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

func TestParseKarmaSegmentsIncludesSubteams(t *testing.T) {
	got := ParseKarmaSegments("<!subteam^S0614TZR72F|@admins> ++++")
	if len(got) != 1 || got[0].Kind != KarmaSegmentSubteam || got[0].SubteamID != "S0614TZR72F" || got[0].SymbolRun != "++++" {
		t.Fatalf("unexpected segment: %+v", got)
	}

	mixed := ParseKarmaSegments("<@U1> ++ <!subteam^S9|@grp> ---")
	if len(mixed) != 2 {
		t.Fatalf("want 2 segments, got %d", len(mixed))
	}
	if mixed[0].Kind != KarmaSegmentUser || mixed[0].UserID != "U1" || mixed[0].SymbolRun != "++" {
		t.Fatalf("seg0: %+v", mixed[0])
	}
	if mixed[1].Kind != KarmaSegmentSubteam || mixed[1].SubteamID != "S9" || mixed[1].SymbolRun != "---" {
		t.Fatalf("seg1: %+v", mixed[1])
	}
}

func TestParseKarmaSegmentsRejectsMixedPlusMinusNoise(t *testing.T) {
	garbage := "<@UBOT> ---++--++++++----×÷xxxooo"
	if seg := ParseKarmaSegments(garbage); len(seg) != 0 {
		t.Fatalf("expected no segments, got %+v", seg)
	}
}

func TestParseKarmaSegmentsAllowsMinusThenWords(t *testing.T) {
	got := ParseKarmaSegments("<@U9> --- thanks")
	if len(got) != 1 || got[0].UserID != "U9" || got[0].SymbolRun != "---" {
		t.Fatalf("unexpected: %+v", got)
	}
}

func TestDedupeKarmaSegments(t *testing.T) {
	in := []KarmaSegment{
		{Kind: KarmaSegmentUser, UserID: "U1", SymbolRun: "++"},
		{Kind: KarmaSegmentUser, UserID: "U1", SymbolRun: "+++"},
	}
	got := DedupeKarmaSegments(in)
	if len(got) != 1 || got[0].SymbolRun != "++" {
		t.Fatalf("want first segment only, got %+v", got)
	}
}
