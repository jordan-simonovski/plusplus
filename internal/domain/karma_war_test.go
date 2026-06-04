package domain

import (
	"context"
	"testing"
	"time"
)

type memStore struct {
	windows map[string]InteractionWindow
}

func newMemStore() *memStore { return &memStore{windows: map[string]InteractionWindow{}} }

func (m *memStore) LoadRecent(_ context.Context, teamID string) (InteractionWindow, error) {
	return m.windows[teamID], nil
}

func (m *memStore) SaveRecent(_ context.Context, teamID string, w InteractionWindow) error {
	m.windows[teamID] = w
	return nil
}

// newTestWar wires a detector with a controllable clock and a deterministic
// picker that echoes the vibe, so tests can assert which pattern fired.
func newTestWar(store InteractionStore, now *time.Time) *KarmaWar {
	w := NewKarmaWar(store)
	w.clock = func() time.Time { return *now }
	w.pick = func(vibe warVibe, _ int) string { return string(vibe) }
	return w
}

func observe(t *testing.T, w *KarmaWar, in Interaction) string {
	t.Helper()
	line, err := w.Observe(context.Background(), "T1", DefaultSnarkLevel, in)
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	return line
}

func TestKarmaWarFirstInteractionIsQuiet(t *testing.T) {
	now := time.Unix(1_000, 0)
	w := newTestWar(newMemStore(), &now)
	if line := observe(t, w, Interaction{Actor: "A", Target: "B", Sign: 1}); line != "" {
		t.Fatalf("expected silence on first interaction, got %q", line)
	}
}

func TestKarmaWarReciprocalVibes(t *testing.T) {
	cases := []struct {
		name      string
		firstSign int
		backSign  int
		wantVibe  warVibe
	}{
		{"feud", -1, -1, vibeFeud},
		{"cartel", 1, 1, vibeCartel},
		{"retaliation_minus_after_plus", 1, -1, vibeRetaliation},
		{"retaliation_plus_after_minus", -1, 1, vibeRetaliation},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Unix(1_000, 0)
			w := newTestWar(newMemStore(), &now)

			if line := observe(t, w, Interaction{Actor: "A", Target: "B", Sign: tc.firstSign}); line != "" {
				t.Fatalf("expected silence on first hit, got %q", line)
			}
			now = now.Add(10 * time.Second)
			got := observe(t, w, Interaction{Actor: "B", Target: "A", Sign: tc.backSign})
			if want := italicize(string(tc.wantVibe)); got != want {
				t.Fatalf("reciprocal: got %q want %q", got, want)
			}
		})
	}
}

func TestKarmaWarRepeatFarmAndPileOn(t *testing.T) {
	cases := []struct {
		name     string
		sign     int
		wantVibe warVibe
	}{
		{"farm", 1, vibeFarm},
		{"pileon", -1, vibePileOn},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Unix(1_000, 0)
			w := newTestWar(newMemStore(), &now)

			for i := 0; i < 2; i++ {
				if line := observe(t, w, Interaction{Actor: "A", Target: "B", Sign: tc.sign}); line != "" {
					t.Fatalf("hit %d should be quiet, got %q", i+1, line)
				}
				now = now.Add(5 * time.Second)
			}
			got := observe(t, w, Interaction{Actor: "A", Target: "B", Sign: tc.sign})
			if want := italicize(string(tc.wantVibe)); got != want {
				t.Fatalf("repeat: got %q want %q", got, want)
			}
		})
	}
}

func TestKarmaWarBurstSpray(t *testing.T) {
	now := time.Unix(1_000, 0)
	w := newTestWar(newMemStore(), &now)

	observe(t, w, Interaction{Actor: "A", Target: "B", Sign: 1})
	now = now.Add(2 * time.Second)
	observe(t, w, Interaction{Actor: "A", Target: "C", Sign: 1})
	now = now.Add(2 * time.Second)

	got := observe(t, w, Interaction{Actor: "A", Target: "D", Sign: 1})
	if want := italicize(string(vibeSpray)); got != want {
		t.Fatalf("burst: got %q want %q", got, want)
	}
}

func TestKarmaWarGroupTrain(t *testing.T) {
	now := time.Unix(1_000, 0)
	w := newTestWar(newMemStore(), &now)

	observe(t, w, Interaction{Actor: "A", Target: "G1", IsGroup: true, Sign: 1})
	now = now.Add(10 * time.Second)
	observe(t, w, Interaction{Actor: "B", Target: "G1", IsGroup: true, Sign: 1})
	now = now.Add(10 * time.Second)

	got := observe(t, w, Interaction{Actor: "C", Target: "G1", IsGroup: true, Sign: 1})
	if want := italicize(string(vibeTrain)); got != want {
		t.Fatalf("train: got %q want %q", got, want)
	}
}

func TestKarmaWarWindowExpiry(t *testing.T) {
	now := time.Unix(1_000, 0)
	w := newTestWar(newMemStore(), &now)

	observe(t, w, Interaction{Actor: "A", Target: "B", Sign: -1})
	now = now.Add(6 * time.Minute) // beyond the 5-minute window

	if line := observe(t, w, Interaction{Actor: "B", Target: "A", Sign: -1}); line != "" {
		t.Fatalf("expected silence after window expiry, got %q", line)
	}
}

func TestKarmaWarCooldownSuppressesRepeatSnark(t *testing.T) {
	now := time.Unix(1_000, 0)
	w := newTestWar(newMemStore(), &now)

	observe(t, w, Interaction{Actor: "A", Target: "B", Sign: -1})
	now = now.Add(5 * time.Second)
	if line := observe(t, w, Interaction{Actor: "B", Target: "A", Sign: -1}); line == "" {
		t.Fatalf("expected reciprocal snark to fire")
	}

	// Another reverse hit within the cooldown must stay quiet.
	now = now.Add(5 * time.Second)
	if line := observe(t, w, Interaction{Actor: "A", Target: "B", Sign: -1}); line != "" {
		t.Fatalf("expected cooldown to suppress snark, got %q", line)
	}

	// Past the cooldown it may fire again.
	now = now.Add(61 * time.Second)
	if line := observe(t, w, Interaction{Actor: "B", Target: "A", Sign: -1}); line == "" {
		t.Fatalf("expected snark to fire again after cooldown")
	}
}

func TestKarmaWarNilDetectorIsNoop(t *testing.T) {
	var w *KarmaWar
	line, err := w.Observe(context.Background(), "T1", DefaultSnarkLevel, Interaction{Actor: "A", Target: "B", Sign: 1})
	if err != nil || line != "" {
		t.Fatalf("nil detector should be a silent no-op, got %q err %v", line, err)
	}
}
