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
	names             NameResolver
	war               *KarmaWar
}

func NewKarmaService(repository KarmaRepository, pickSnark SnarkPicker, maxKarmaPerAction int, names NameResolver) *KarmaService {
	if maxKarmaPerAction < 1 {
		maxKarmaPerAction = 5
	}

	return &KarmaService{
		repository:        repository,
		pickSnark:         pickSnark,
		maxKarmaPerAction: maxKarmaPerAction,
		names:             names,
	}
}

// WithKarmaWar attaches the quick-succession snark detector. Optional: a service
// without it behaves exactly as before.
func (s *KarmaService) WithKarmaWar(war *KarmaWar) *KarmaService {
	s.war = war
	return s
}

// resolveName returns the user's display name, degrading to fallback (a mention)
// when no resolver is configured or the lookup fails. Display, never persist.
func (s *KarmaService) resolveName(ctx context.Context, teamID, userID, fallback string) string {
	if s.names == nil {
		return fallback
	}
	name, err := s.names.DisplayName(ctx, teamID, userID)
	if err != nil || name == "" {
		return fallback
	}
	return name
}

func (s *KarmaService) HandleAction(ctx context.Context, action KarmaAction) (KarmaResult, error) {
	if action.TargetUserID == "" || action.TargetHandle == "" {
		return KarmaResult{ShouldPersist: false, Message: ""}, nil
	}

	outcome := EvaluateKarmaActionWithLimits(EvaluateInput{
		ActorUserID:  action.ActorUserID,
		TargetUserID: action.TargetUserID,
		SymbolRun:    action.SymbolRun,
	}, minSymbolCount, s.maxKarmaPerAction+1)

	if outcome.Kind == OutcomeReject {
		return s.handleRejection(ctx, outcome.Reason, action), nil
	}

	record, err := s.repository.ApplyDelta(ctx, action.TeamID, action.TargetUserID, outcome.Delta)
	if err != nil {
		return KarmaResult{}, err
	}

	targetName := s.resolveName(ctx, action.TeamID, action.TargetUserID, action.TargetHandle)
	message := FormatKarmaAppliedMessage(targetName, outcome.Delta, record, outcome.Capped, s.maxKarmaPerAction, action.SnarkLevel)

	// Group fan-out records one group-level interaction elsewhere; recording each
	// member here would falsely trip burst detection.
	if !action.GroupBroadcast {
		sign := 1
		if outcome.Delta < 0 {
			sign = -1
		}
		// War snark is best-effort flavor: a store error must not fail the karma reply.
		if line, _ := s.war.Observe(ctx, action.TeamID, action.SnarkLevel, Interaction{
			Actor:  action.ActorUserID,
			Target: action.TargetUserID,
			Sign:   sign,
		}); line != "" {
			message += "\n" + line
		}
	}

	return KarmaResult{
		ShouldPersist: true,
		Message:       message,
	}, nil
}

// ObserveGroupAward records a user-group karma action and returns a snark line
// (already italicized, "" when nothing fires) for a karma train on that group.
func (s *KarmaService) ObserveGroupAward(ctx context.Context, teamID, actorID, groupID string, sign, snarkLevel int) (string, error) {
	return s.war.Observe(ctx, teamID, snarkLevel, Interaction{
		Actor:   actorID,
		Target:  groupID,
		IsGroup: true,
		Sign:    sign,
	})
}

func (s *KarmaService) HandleLeaderboard(ctx context.Context, request LeaderboardRequest) (KarmaResult, error) {
	records, err := s.repository.GetLeaderboard(ctx, request.TeamID, leaderboardScanLimit)
	if err != nil {
		return KarmaResult{}, err
	}

	if len(records) > leaderboardResultLimit {
		records = records[:leaderboardResultLimit]
	}

	names := make([]string, len(records))
	for i, entry := range records {
		names[i] = s.resolveName(ctx, request.TeamID, entry.UserID, "<@"+entry.UserID+">")
	}

	return KarmaResult{
		ShouldPersist: false,
		Message:       FormatLeaderboardMessage(records, names),
	}, nil
}

func (s *KarmaService) handleRejection(ctx context.Context, reason RejectionReason, action KarmaAction) KarmaResult {
	if action.GroupBroadcast {
		switch reason {
		case RejectionSelfAward, RejectionSelfRemove:
			targetName := s.resolveName(ctx, action.TeamID, action.TargetUserID, action.TargetHandle)
			return KarmaResult{ShouldPersist: false, Message: FormatGroupSelfKarmaRejection(targetName, reason)}
		}
	}
	switch reason {
	case RejectionSelfAward, RejectionSelfRemove:
		return KarmaResult{ShouldPersist: false, Message: s.pickSnark(reason, action.SnarkLevel)}
	case RejectionInvalidFormat:
		return KarmaResult{ShouldPersist: false, Message: ""}
	default:
		return KarmaResult{ShouldPersist: false, Message: ""}
	}
}

var ErrUnsupportedReason = errors.New("unsupported rejection reason")
