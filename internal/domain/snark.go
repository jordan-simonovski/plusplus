package domain

import (
	"fmt"
	"math/rand"
	"time"
)

var snarkMessages = map[RejectionReason][]string{
	RejectionSelfAward: {
		"Nice try. You cannot farm karma from yourself.",
		"Self-award denied.",
		"No. Earn karma from someone else.",
		"Denied. Your alt account does not count if it is still you.",
		"That was an inside job. Blocked.",
		"Self-praise is free. Karma is not.",
		"Request rejected. The jury was also you.",
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
	},
	RejectionSelfRemove: {
		"Self-minus is disabled.",
		"Self-penalty denied.",
		"You cannot dock your own karma.",
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
		"Self-remove denied. Save the theatrics for release notes.",
	},
}

var cappedAwardSnarkMessages = []string{
	"Easy there, turbo. Nobody gets more than %d karma per hit.",
	"Nice enthusiasm. The karma speed limit is %d.",
	"Calm down, forklift. We cap heavy lifting at %d karma.",
	"That burst exceeds spec. Clamped to %d.",
	"Ambition noted. Throughput limited to %d karma.",
	"Great hustle. This endpoint tops out at %d.",
	"Overclock denied. Maximum output is %d karma.",
	"That request came in hot. We shaved it to %d.",
	"Too much sauce. Capped at %d karma.",
	"Love the energy. Still capped at %d.",
}

func RandomSnarkPicker() SnarkPicker {
	seed := rand.New(rand.NewSource(time.Now().UnixNano()))
	return func(reason RejectionReason) string {
		list, ok := snarkMessages[reason]
		if !ok || len(list) == 0 {
			return "Invalid karma command."
		}
		return list[seed.Intn(len(list))]
	}
}

func RandomCappedAwardSnark(maxKarmaPerAction int) string {
	if len(cappedAwardSnarkMessages) == 0 {
		return "Buzzkill mode enabled."
	}

	seed := rand.New(rand.NewSource(time.Now().UnixNano()))
	template := cappedAwardSnarkMessages[seed.Intn(len(cappedAwardSnarkMessages))]
	return fmt.Sprintf(template, maxKarmaPerAction)
}
