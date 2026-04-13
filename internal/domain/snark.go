package domain

import (
	"fmt"
	"math/rand"
	"time"
)

const (
	MinSnarkLevel     = 1
	MaxSnarkLevel     = 10
	DefaultSnarkLevel = 5
)

// selfAwardFlat is ordered mild → spicy; tierSlice maps levels 1–10 to disjoint ranges.
var selfAwardFlat = []string{
	"Self-award denied.",
	"No.",
	"Nice try. You cannot farm karma from yourself.",
	"Self-praise is free. Karma is not.",
	"Denied. Your alt account does not count if it is still you.",
	"That was an inside job. Blocked.",
	"Bold move. Still no.",
	"You cannot invoice the system for your own hype.",
	"Loopback karma detected. Packet dropped.",
	"Write better messages, not better self-reviews.",
	"This bot is not your performance review committee.",
	"Rejected. Please involve at least one other human.",
	"You tried to mint karma locally. Central bank said no.",
	"Self-award blocked. Nice syntax though.",
	"Denied at compile time: circular praise dependency.",
	"You cannot ACK your own applause.",
	"Great confidence. Wrong endpoint.",
	"Karma cannot be bootstrapped from ego.",
	"That is not collaboration, that is recursion.",
	"Nope. Please seek external validation like the rest of us.",
	"Rejected. The mirror is not a teammate.",
	"Self-award denied. Good attempt, bad protocol.",
	"Transaction rolled back: sender equals receiver.",
	"You cannot plus-plus yourself into credibility.",
	"Denied. This code path requires witnesses.",
	"Your PR approval matrix cannot be size one.",
	"Audit log says: nice hustle, still invalid.",
	"Blocked. Karma laundering is frowned upon.",
}

var selfRemoveFlat = []string{
	"Self-minus is disabled.",
	"No.",
	"You cannot dock your own karma.",
	"Self-penalty denied.",
	"Denied. We are not running emotional chaos engineering.",
	"Self-sabotage blocked. Try coffee instead.",
	"No dramatic exits today.",
	"Request rejected. Please stop rage-deploying on yourself.",
	"You cannot downvote your own existence.",
	"Denied. This is karma tracking, not penance software.",
	"Self-remove blocked. Keep your meltdown off the main branch.",
	"You cannot file an incident against yourself and auto-close it.",
	"Negative self-feedback loop detected and interrupted.",
	"Denied. Please debug the problem, not your soul.",
	"That is not humility, that is bad state management.",
	"Self-minus rejected. Logs show excessive drama throughput.",
	"Request blocked. We do not support recursive guilt.",
	"Denied. This is not a sadness API.",
	"Self-penalty rejected. Go touch grass, then ship.",
	"You cannot amortize regret with minus signs.",
	"Rollback complete: self-destruction disabled.",
	"Denied. Please stop load-testing your self-esteem.",
	"Not allowed. We protect uptime and users, including you.",
	"Self-remove blocked by common sense middleware.",
	"Rejected. Catastrophic self-commentary prevented.",
	"Denied. The hot path is for work, not self-critique.",
	"You cannot open a ticket against your own username here.",
	"Self-minus denied. This is a team sport.",
	"Blocked. Karma bankruptcy is not supported.",
	"Rejected. Try a bugfix, not a self-ban.",
	"Denied. Your personal outage has no runbook.",
}

// cappedAwardSnarkByLevel: low levels stay dry; high levels use the original voice.
var cappedAwardSnarkByLevel = [10][]string{
	{"Capped at %d karma."},
	{"Award limited to %d karma."},
	{"Clamped to %d karma."},
	{"That burst exceeds spec. Clamped to %d."},
	{"Ambition noted. Throughput limited to %d karma."},
	{"Great hustle. This endpoint tops out at %d."},
	{"Overclock denied. Maximum output is %d karma."},
	{"That request came in hot. We shaved it to %d."},
	{"Nice enthusiasm. The karma speed limit is %d."},
	{
		"Easy there, turbo. Nobody gets more than %d karma per hit.",
		"Calm down, forklift. We cap heavy lifting at %d karma.",
		"Too much sauce. Capped at %d karma.",
		"Love the energy. Still capped at %d.",
	},
}

func NormalizeSnarkLevel(n int) int {
	if n >= MinSnarkLevel && n <= MaxSnarkLevel {
		return n
	}
	return DefaultSnarkLevel
}

func tierSlice(flat []string, level int) []string {
	if len(flat) == 0 {
		return nil
	}
	per := len(flat) / 10
	if per < 1 {
		per = 1
	}
	if level < 1 {
		level = 1
	}
	if level > 10 {
		level = 10
	}
	start := (level - 1) * per
	if start >= len(flat) {
		return flat[len(flat)-1:]
	}
	end := level * per
	if level == 10 || end > len(flat) {
		end = len(flat)
	}
	return flat[start:end]
}

func snarkPool(reason RejectionReason, level int) []string {
	level = NormalizeSnarkLevel(level)
	switch reason {
	case RejectionSelfAward:
		return tierSlice(selfAwardFlat, level)
	case RejectionSelfRemove:
		return tierSlice(selfRemoveFlat, level)
	default:
		return nil
	}
}

func RandomSnarkPicker() SnarkPicker {
	seed := rand.New(rand.NewSource(time.Now().UnixNano()))
	return func(reason RejectionReason, snarkLevel int) string {
		list := snarkPool(reason, snarkLevel)
		if len(list) == 0 {
			return "Invalid karma command."
		}
		return list[seed.Intn(len(list))]
	}
}

func RandomCappedAwardSnark(maxKarmaPerAction int, snarkLevel int) string {
	level := NormalizeSnarkLevel(snarkLevel)
	idx := level - 1
	templates := cappedAwardSnarkByLevel[idx]
	if len(templates) == 0 {
		return "Buzzkill mode enabled."
	}

	seed := rand.New(rand.NewSource(time.Now().UnixNano()))
	template := templates[seed.Intn(len(templates))]
	return fmt.Sprintf(template, maxKarmaPerAction)
}
