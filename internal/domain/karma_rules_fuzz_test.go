package domain

import "testing"

func FuzzEvaluateKarmaActionWithLimits(f *testing.F) {
	for _, seed := range []struct {
		run  string
		self bool
	}{{"++", false}, {"--", false}, {"+++++++++", false}, {"—", false}, {"++", true}, {"--", true}, {"+-", false}, {"", false}, {"  ++  ", false}} {
		f.Add(seed.run, seed.self)
	}
	f.Fuzz(func(t *testing.T, run string, self bool) {
		target := "B"
		if self {
			target = "A"
		}
		out := EvaluateKarmaActionWithLimits(EvaluateInput{ActorUserID: "A", TargetUserID: target, SymbolRun: run}, minSymbolCount, maxSymbolCount)
		if out.Kind != OutcomeApply {
			return
		}
		if self {
			t.Fatalf("run %q: self karma was applied: %+v", run, out)
		}
		if d := max(out.Delta, -out.Delta); d < 1 || d > maxSymbolCount-1 {
			t.Fatalf("run %q: delta %d outside 1..%d", run, out.Delta, maxSymbolCount-1)
		}
	})
}
