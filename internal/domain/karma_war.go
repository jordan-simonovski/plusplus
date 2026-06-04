package domain

import (
	"context"
	"time"
)

// Interaction is a single applied karma act: Actor moved Target's karma by Sign
// (+1 award, -1 remove). IsGroup marks a user-group (subteam) target. UnixAt is
// set by the detector at Observe time.
type Interaction struct {
	Actor   string `json:"a"`
	Target  string `json:"t"`
	IsGroup bool   `json:"g,omitempty"`
	Sign    int    `json:"s"`
	UnixAt  int64  `json:"u"`
}

// InteractionWindow is a team's recent karma activity plus per-pattern cooldown
// timestamps. It is the entire persisted state for war detection.
type InteractionWindow struct {
	Recent    []Interaction    `json:"recent"`
	Cooldowns map[string]int64 `json:"cooldowns,omitempty"`
}

// InteractionStore persists one InteractionWindow per team. Implementations are
// best-effort and last-writer-wins; this is comedic flavor, not accounting.
type InteractionStore interface {
	LoadRecent(ctx context.Context, teamID string) (InteractionWindow, error)
	SaveRecent(ctx context.Context, teamID string, window InteractionWindow) error
}

type warVibe string

const (
	vibeCartel      warVibe = "cartel"      // mutual award loop
	vibeFeud        warVibe = "feud"        // mutual removal loop
	vibeRetaliation warVibe = "retaliation" // reciprocal, mismatched signs
	vibeTrain       warVibe = "train"       // repeated awards to the same group
	vibeFarm        warVibe = "farm"        // same actor awarding same target repeatedly
	vibePileOn      warVibe = "pileon"      // same actor docking same target repeatedly
	vibeSpray       warVibe = "spray"       // same actor hitting many targets in a burst
)

const (
	defaultWarWindow      = 5 * time.Minute
	defaultWarCooldown    = 60 * time.Second
	maxTrackedInteraction = 64

	reciprocalThreshold  = 1 // one prior reverse hit → fire on the reply
	repeatThreshold      = 2 // two prior identical hits → fire on the 3rd
	trainThreshold       = 2 // two prior awards to the group → fire on the 3rd
	burstDistinctTargets = 3 // three distinct targets in the window
)

type warSnarkPicker func(vibe warVibe, level int) string

// KarmaWar detects reciprocal/repeat/burst karma activity and returns a snark
// line when a pattern fires. A nil *KarmaWar is a valid no-op.
type KarmaWar struct {
	store    InteractionStore
	window   time.Duration
	cooldown time.Duration
	pick     warSnarkPicker
	clock    func() time.Time
}

func NewKarmaWar(store InteractionStore) *KarmaWar {
	return &KarmaWar{
		store:    store,
		window:   defaultWarWindow,
		cooldown: defaultWarCooldown,
		pick:     defaultWarSnark,
		clock:    time.Now,
	}
}

// Observe records the interaction and returns a snark line (already italicized)
// when a pattern fires and its cooldown has elapsed, or "" otherwise. Errors
// from the store are returned; callers may safely ignore them (best-effort).
func (w *KarmaWar) Observe(ctx context.Context, teamID string, level int, in Interaction) (string, error) {
	if w == nil || w.store == nil {
		return "", nil
	}

	now := w.clock()
	in.UnixAt = now.Unix()

	win, err := w.store.LoadRecent(ctx, teamID)
	if err != nil {
		return "", err
	}

	cutoff := now.Add(-w.window).Unix()
	recent := trimOlderThan(win.Recent, cutoff)

	verdict := classifyWar(in, recent)

	var snark string
	if verdict.fires {
		coolSec := int64(w.cooldown / time.Second)
		if now.Unix()-win.Cooldowns[verdict.key] >= coolSec {
			if line := w.pick(verdict.vibe, level); line != "" {
				snark = italicize(line)
				if win.Cooldowns == nil {
					win.Cooldowns = make(map[string]int64)
				}
				win.Cooldowns[verdict.key] = now.Unix()
			}
		}
	}

	recent = append(recent, in)
	if len(recent) > maxTrackedInteraction {
		recent = recent[len(recent)-maxTrackedInteraction:]
	}
	win.Recent = recent
	win.Cooldowns = trimCooldowns(win.Cooldowns, cutoff)

	if err := w.store.SaveRecent(ctx, teamID, win); err != nil {
		return "", err
	}
	return snark, nil
}

type warVerdict struct {
	fires bool
	vibe  warVibe
	key   string
}

// classifyWar inspects the current interaction against the (already
// window-trimmed) recent history and returns the highest-priority pattern that
// fires. Priority: reciprocal > group train > rapid repeat > burst.
func classifyWar(cur Interaction, recent []Interaction) warVerdict {
	if !cur.IsGroup {
		for _, p := range recent {
			if !p.IsGroup && p.Actor == cur.Target && p.Target == cur.Actor {
				vibe := vibeRetaliation
				switch {
				case p.Sign > 0 && cur.Sign > 0:
					vibe = vibeCartel
				case p.Sign < 0 && cur.Sign < 0:
					vibe = vibeFeud
				}
				return warVerdict{fires: true, vibe: vibe, key: "recip:" + pairKey(cur.Actor, cur.Target)}
			}
		}
	}

	if cur.IsGroup {
		count := 0
		for _, p := range recent {
			if p.IsGroup && p.Target == cur.Target {
				count++
			}
		}
		if count >= trainThreshold {
			return warVerdict{fires: true, vibe: vibeTrain, key: "train:" + cur.Target}
		}
	}

	if !cur.IsGroup {
		count := 0
		for _, p := range recent {
			if !p.IsGroup && p.Actor == cur.Actor && p.Target == cur.Target && sameSign(p.Sign, cur.Sign) {
				count++
			}
		}
		if count >= repeatThreshold {
			vibe := vibeFarm
			if cur.Sign < 0 {
				vibe = vibePileOn
			}
			return warVerdict{fires: true, vibe: vibe, key: "repeat:" + cur.Actor + ">" + cur.Target}
		}
	}

	if !cur.IsGroup {
		targets := map[string]struct{}{cur.Target: {}}
		for _, p := range recent {
			if !p.IsGroup && p.Actor == cur.Actor {
				targets[p.Target] = struct{}{}
			}
		}
		if len(targets) >= burstDistinctTargets {
			return warVerdict{fires: true, vibe: vibeSpray, key: "burst:" + cur.Actor}
		}
	}

	return warVerdict{}
}

// pairKey is order-independent so A→B and B→A share one reciprocal cooldown.
func pairKey(a, b string) string {
	if a <= b {
		return a + "|" + b
	}
	return b + "|" + a
}

func sameSign(a, b int) bool { return (a >= 0) == (b >= 0) }

func trimOlderThan(in []Interaction, cutoff int64) []Interaction {
	out := in[:0:0]
	for _, it := range in {
		if it.UnixAt >= cutoff {
			out = append(out, it)
		}
	}
	return out
}

func trimCooldowns(in map[string]int64, cutoff int64) map[string]int64 {
	if len(in) == 0 {
		return in
	}
	for k, at := range in {
		if at < cutoff {
			delete(in, k)
		}
	}
	if len(in) == 0 {
		return nil
	}
	return in
}
