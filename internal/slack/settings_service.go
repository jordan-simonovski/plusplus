package slack

import (
	"context"
	"fmt"

	"plusplus/internal/domain"
)

type ChannelSettingsStore interface {
	GetChannelSettings(ctx context.Context, teamID string, channelID string) (replyMode string, snarkLevel int, err error)
	SetReplyMode(ctx context.Context, teamID string, channelID string, actorUserID string, replyMode string) error
	SetSnarkLevel(ctx context.Context, teamID string, channelID string, actorUserID string, snarkLevel int) error
}

type ChannelSettingsService struct {
	store ChannelSettingsStore
}

func NewChannelSettingsService(store ChannelSettingsStore) *ChannelSettingsService {
	return &ChannelSettingsService{store: store}
}

func (s *ChannelSettingsService) GetChannelSettings(ctx context.Context, teamID string, channelID string) (ReplyMode, int, error) {
	modeStr, level, err := s.store.GetChannelSettings(ctx, teamID, channelID)
	if err != nil {
		return "", 0, err
	}

	switch modeStr {
	case string(ReplyModeThread):
		return ReplyModeThread, level, nil
	case string(ReplyModeChannel):
		return ReplyModeChannel, level, nil
	default:
		return ReplyModeThread, level, nil
	}
}

func (s *ChannelSettingsService) GetReplyMode(ctx context.Context, teamID string, channelID string) (ReplyMode, error) {
	mode, _, err := s.GetChannelSettings(ctx, teamID, channelID)
	return mode, err
}

func (s *ChannelSettingsService) GetSnarkLevel(ctx context.Context, teamID string, channelID string) (int, error) {
	_, level, err := s.GetChannelSettings(ctx, teamID, channelID)
	return level, err
}

func (s *ChannelSettingsService) SetReplyMode(ctx context.Context, teamID string, channelID string, actorUserID string, mode ReplyMode) (string, error) {
	if mode != ReplyModeThread && mode != ReplyModeChannel {
		return "", fmt.Errorf("invalid reply mode: %s", mode)
	}

	if err := s.store.SetReplyMode(ctx, teamID, channelID, actorUserID, string(mode)); err != nil {
		return "", err
	}

	return fmt.Sprintf("Reply mode set to %s for this channel.", mode), nil
}

func (s *ChannelSettingsService) SetSnarkLevel(ctx context.Context, teamID string, channelID string, actorUserID string, level int) (string, error) {
	if level < domain.MinSnarkLevel || level > domain.MaxSnarkLevel {
		return "", fmt.Errorf("invalid snark level: %d (allowed %d–%d)", level, domain.MinSnarkLevel, domain.MaxSnarkLevel)
	}

	if err := s.store.SetSnarkLevel(ctx, teamID, channelID, actorUserID, level); err != nil {
		return "", err
	}

	return fmt.Sprintf("Snark level set to %d for this channel.", level), nil
}
