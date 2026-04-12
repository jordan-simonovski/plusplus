package domain

import (
	"context"
	"errors"
)

const (
	leaderboardScanLimit   = 25
	leaderboardResultLimit = 5
)

type KarmaService struct {
	repository        KarmaRepository
	pickSnark         SnarkPicker
	maxKarmaPerAction int
}

func NewKarmaService(repository KarmaRepository, pickSnark SnarkPicker, maxKarmaPerAction int) *KarmaService {
	if maxKarmaPerAction < 1 {
		maxKarmaPerAction = 5
	}

	return &KarmaService{
		repository:        repository,
		pickSnark:         pickSnark,
		maxKarmaPerAction: maxKarmaPerAction,
	}
}

func (s *KarmaService) HandleAction(ctx context.Context, action KarmaAction) (KarmaResult, error) {
	if action.TargetUserID == "" || action.TargetHandle == "" {
		return KarmaResult{
			ShouldPersist: false,
			Message:       "Invalid karma command. Missing target user.",
		}, nil
	}

	outcome := EvaluateKarmaActionWithLimits(EvaluateInput{
		ActorUserID:  action.ActorUserID,
		TargetUserID: action.TargetUserID,
		SymbolRun:    action.SymbolRun,
	}, minSymbolCount, s.maxKarmaPerAction+1)

	if outcome.Kind == OutcomeReject {
		return s.handleRejection(outcome.Reason), nil
	}

	record, err := s.repository.ApplyDelta(ctx, action.TeamID, action.TargetUserID, outcome.Delta)
	if err != nil {
		return KarmaResult{}, err
	}

	return KarmaResult{
		ShouldPersist: true,
		Message:       FormatKarmaAppliedMessage(action.TargetHandle, outcome.Delta, record, outcome.Capped, s.maxKarmaPerAction),
	}, nil
}

func (s *KarmaService) HandleLeaderboard(ctx context.Context, request LeaderboardRequest) (KarmaResult, error) {
	records, err := s.repository.GetLeaderboard(ctx, request.TeamID, leaderboardScanLimit)
	if err != nil {
		return KarmaResult{}, err
	}

	if len(records) > leaderboardResultLimit {
		records = records[:leaderboardResultLimit]
	}

	return KarmaResult{
		ShouldPersist: false,
		Message:       FormatLeaderboardMessage(records),
	}, nil
}

func (s *KarmaService) handleRejection(reason RejectionReason) KarmaResult {
	switch reason {
	case RejectionSelfAward, RejectionSelfRemove:
		return KarmaResult{ShouldPersist: false, Message: s.pickSnark(reason)}
	case RejectionInvalidFormat:
		return KarmaResult{ShouldPersist: false, Message: "Invalid karma command. Use @user ++ to @user ++++++ or -- to ------."}
	default:
		return KarmaResult{ShouldPersist: false, Message: "Invalid karma command."}
	}
}

var ErrUnsupportedReason = errors.New("unsupported rejection reason")
