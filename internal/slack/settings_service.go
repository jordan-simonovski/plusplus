package slack

import (
	"context"
	"fmt"
)

type ChannelSettingsStore interface {
	GetReplyMode(ctx context.Context, teamID string, channelID string) (string, error)
	SetReplyMode(ctx context.Context, teamID string, channelID string, actorUserID string, replyMode string) error
}

type ChannelSettingsService struct {
	store ChannelSettingsStore
}

func NewChannelSettingsService(store ChannelSettingsStore) *ChannelSettingsService {
	return &ChannelSettingsService{store: store}
}

func (s *ChannelSettingsService) GetReplyMode(ctx context.Context, teamID string, channelID string) (ReplyMode, error) {
	mode, err := s.store.GetReplyMode(ctx, teamID, channelID)
	if err != nil {
		return "", err
	}

	switch mode {
	case string(ReplyModeThread):
		return ReplyModeThread, nil
	case string(ReplyModeChannel):
		return ReplyModeChannel, nil
	default:
		return ReplyModeThread, nil
	}
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
