package domain

import "context"

type RejectionReason string

const (
	RejectionInvalidFormat RejectionReason = "invalid_format"
	RejectionSelfAward     RejectionReason = "self_award"
	RejectionSelfRemove    RejectionReason = "self_remove"
)

type KarmaOutcomeKind string

const (
	OutcomeApply  KarmaOutcomeKind = "apply"
	OutcomeReject KarmaOutcomeKind = "reject"
)

type KarmaRuleOutcome struct {
	Kind   KarmaOutcomeKind
	Delta  int
	Capped bool
	Reason RejectionReason
}

type EvaluateInput struct {
	ActorUserID  string
	TargetUserID string
	SymbolRun    string
}

type KarmaRecord struct {
	TeamID       string
	UserID       string
	KarmaTotal   int
	KarmaMax     int
	LastActivity string
}

type KarmaAction struct {
	TeamID       string
	ActorUserID  string
	TargetUserID string
	TargetHandle string
	SymbolRun    string
	// SnarkLevel is 1–10; 0 means use DefaultSnarkLevel (channel default applies at the transport layer).
	SnarkLevel int
	// GroupBroadcast uses short, fixed copy for self-denial instead of random snark (user group karma).
	GroupBroadcast bool
}

type LeaderboardRequest struct {
	TeamID string
}

type KarmaResult struct {
	ShouldPersist bool
	Message       string
}

type KarmaRepository interface {
	ApplyDelta(ctx context.Context, teamID string, userID string, delta int) (KarmaRecord, error)
	GetLeaderboard(ctx context.Context, teamID string, limit int) ([]KarmaRecord, error)
}

type SnarkPicker func(reason RejectionReason, snarkLevel int) string
