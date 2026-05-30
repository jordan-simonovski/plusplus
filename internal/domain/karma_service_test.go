package domain

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestKarmaServiceHandleAction(t *testing.T) {
	repo := &fakeRepo{
		record: KarmaRecord{
			TeamID:    "T1",
			UserID:    "U2",
			KarmaTotal: 7,
			KarmaMax:   10,
		},
	}
	service := NewKarmaService(repo, fakeSnark, 5, nil)

	result, err := service.HandleAction(context.Background(), KarmaAction{
		TeamID:       "T1",
		ActorUserID:  "U1",
		TargetUserID: "U2",
		TargetHandle: "<@U2>",
		SymbolRun:    "+++",
	})
	if err != nil {
		t.Fatalf("handle action failed: %v", err)
	}

	if !result.ShouldPersist {
		t.Fatalf("expected persistence")
	}

	if result.Message == "" {
		t.Fatalf("expected non-empty message")
	}
}

func TestKarmaServiceHandleActionUsesResolvedName(t *testing.T) {
	repo := &fakeRepo{record: KarmaRecord{TeamID: "T1", UserID: "U2", KarmaTotal: 7}}
	service := NewKarmaService(repo, fakeSnark, 5, &fakeNames{name: "Jane Doe"})

	result, err := service.HandleAction(context.Background(), KarmaAction{
		TeamID:       "T1",
		ActorUserID:  "U1",
		TargetUserID: "U2",
		TargetHandle: "<@U2>",
		SymbolRun:    "++",
	})
	if err != nil {
		t.Fatalf("handle action failed: %v", err)
	}
	if result.Message != "_*Jane Doe* gained 1 karma. Total: 7._" {
		t.Fatalf("expected resolved name in message, got %q", result.Message)
	}
}

func TestKarmaServiceHandleActionFallsBackToMentionOnResolverError(t *testing.T) {
	repo := &fakeRepo{record: KarmaRecord{TeamID: "T1", UserID: "U2", KarmaTotal: 7}}
	service := NewKarmaService(repo, fakeSnark, 5, &fakeNames{err: errResolve})

	result, err := service.HandleAction(context.Background(), KarmaAction{
		TeamID:       "T1",
		ActorUserID:  "U1",
		TargetUserID: "U2",
		TargetHandle: "<@U2>",
		SymbolRun:    "++",
	})
	if err != nil {
		t.Fatalf("handle action failed: %v", err)
	}
	if result.Message != "_*<@U2>* gained 1 karma. Total: 7._" {
		t.Fatalf("expected mention fallback, got %q", result.Message)
	}
}

func TestKarmaServiceInvalidFormatIsSilent(t *testing.T) {
	service := NewKarmaService(&fakeRepo{}, fakeSnark, 5, nil)

	result, err := service.HandleAction(context.Background(), KarmaAction{
		TeamID:       "T1",
		ActorUserID:  "U1",
		TargetUserID: "U2",
		TargetHandle: "<@U2>",
		SymbolRun:    "+",
	})
	if err != nil {
		t.Fatalf("handle action failed: %v", err)
	}
	if result.ShouldPersist {
		t.Fatalf("expected no persistence")
	}
	if result.Message != "" {
		t.Fatalf("expected no client-visible message, got %q", result.Message)
	}
}

func TestKarmaServiceRejectsSelfAwardWithSnark(t *testing.T) {
	service := NewKarmaService(&fakeRepo{}, fakeSnark, 5, nil)

	result, err := service.HandleAction(context.Background(), KarmaAction{
		TeamID:       "T1",
		ActorUserID:  "U1",
		TargetUserID: "U1",
		TargetHandle: "<@U1>",
		SymbolRun:    "++",
	})
	if err != nil {
		t.Fatalf("handle action failed: %v", err)
	}

	if result.ShouldPersist {
		t.Fatalf("expected no persistence")
	}

	want := fmt.Sprintf("snark:%s:%d", RejectionSelfAward, DefaultSnarkLevel)
	if result.Message != want {
		t.Fatalf("unexpected snark message: got %q want %q", result.Message, want)
	}
}

func TestKarmaServiceGroupBroadcastSelfUsesPlainLine(t *testing.T) {
	service := NewKarmaService(&fakeRepo{}, fakeSnark, 5, nil)

	result, err := service.HandleAction(context.Background(), KarmaAction{
		TeamID:         "T1",
		ActorUserID:    "U1",
		TargetUserID:   "U1",
		TargetHandle:   "<@U1>",
		SymbolRun:      "++",
		GroupBroadcast: true,
	})
	if err != nil {
		t.Fatalf("handle action failed: %v", err)
	}
	if result.ShouldPersist {
		t.Fatalf("expected no persistence")
	}
	want := FormatGroupSelfKarmaRejection("<@U1>", RejectionSelfAward)
	if result.Message != want {
		t.Fatalf("unexpected message: got %q want %q", result.Message, want)
	}
}

func TestKarmaServiceLeaderboard(t *testing.T) {
	repo := &fakeRepo{
		leaderboard: []KarmaRecord{
			{TeamID: "T1", UserID: "U9", KarmaTotal: 14},
			{TeamID: "T1", UserID: "U7", KarmaTotal: 12},
		},
	}
	service := NewKarmaService(repo, fakeSnark, 5, nil)

	result, err := service.HandleLeaderboard(context.Background(), LeaderboardRequest{TeamID: "T1"})
	if err != nil {
		t.Fatalf("handle leaderboard failed: %v", err)
	}

	if result.ShouldPersist {
		t.Fatalf("leaderboard should not persist")
	}

	want := "All-time karma leaderboard\n1. *_<@U9>_* - 14\n2. *_<@U7>_* - 12"
	if result.Message != want {
		t.Fatalf("unexpected leaderboard message: %q", result.Message)
	}
}

type fakeRepo struct {
	record      KarmaRecord
	leaderboard []KarmaRecord
}

func (f *fakeRepo) ApplyDelta(_ context.Context, _ string, _ string, _ int) (KarmaRecord, error) {
	return f.record, nil
}

func (f *fakeRepo) GetLeaderboard(_ context.Context, _ string, _ int) ([]KarmaRecord, error) {
	return f.leaderboard, nil
}

func fakeSnark(reason RejectionReason, snarkLevel int) string {
	return fmt.Sprintf("snark:%s:%d", reason, NormalizeSnarkLevel(snarkLevel))
}

var errResolve = errors.New("resolve failed")

type fakeNames struct {
	name string
	err  error
}

func (f *fakeNames) DisplayName(_ context.Context, _ string, _ string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.name, nil
}
