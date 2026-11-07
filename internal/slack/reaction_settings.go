package slack

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"plusplus/internal/domain"
)

const maxEmojisPerTier = 10

type ReactionOverridesStore interface {
	GetReactionOverrides(ctx context.Context, teamID, channelID string) (channel, workspace domain.TierOverrides, err error)
	SetReactionOverrides(ctx context.Context, teamID, channelID string, workspace bool, actorUserID string, overrides domain.TierOverrides) error
}

// ReactionSettingsService resolves and edits the emoji -> karma tier maps.
// Channel overrides win over workspace overrides, which win over the defaults.
type ReactionSettingsService struct {
	store ReactionOverridesStore
}

func NewReactionSettingsService(store ReactionOverridesStore) *ReactionSettingsService {
	return &ReactionSettingsService{store: store}
}

// ReactionTiers returns the effective emoji -> tier map for a channel.
func (s *ReactionSettingsService) ReactionTiers(ctx context.Context, teamID, channelID string) (map[string]int, error) {
	channel, workspace, err := s.store.GetReactionOverrides(ctx, teamID, channelID)
	if err != nil {
		return nil, err
	}
	return domain.ResolveReactionTiers(workspace, channel), nil
}

func (s *ReactionSettingsService) Describe(ctx context.Context, teamID, channelID string) (string, error) {
	tiers, err := s.ReactionTiers(ctx, teamID, channelID)
	if err != nil {
		return "", err
	}
	return "*Karma reactions in this channel*\n" + domain.FormatReactionTiers(tiers) +
		"\nChange one with `/plusplus-settings emoji <tier> :emoji:` (add `workspace` before the tier to change it everywhere).", nil
}

// SetTier points a tier at new emojis for the channel (or workspace). The emojis are
// removed from any other tier at the same level so each emoji means one thing.
func (s *ReactionSettingsService) SetTier(ctx context.Context, teamID, channelID string, workspace bool, actorUserID, tierArg string, emojiArgs []string) (string, error) {
	tier, err := strconv.Atoi(tierArg)
	if err != nil || !domain.ValidReactionTier(tier) {
		return "", fmt.Errorf("tier must be -%d to -1 or 1 to %d", domain.MaxReactionTier, domain.MaxReactionTier)
	}
	if len(emojiArgs) == 0 || len(emojiArgs) > maxEmojisPerTier {
		return "", fmt.Errorf("give 1 to %d emojis, like :fire:", maxEmojisPerTier)
	}
	emojis := make([]string, 0, len(emojiArgs))
	for _, arg := range emojiArgs {
		emoji := domain.NormalizeEmoji(arg)
		if emoji == "" {
			return "", fmt.Errorf("%q isn't an emoji name, use the :name: form", arg)
		}
		emojis = append(emojis, emoji)
	}

	channel, ws, err := s.store.GetReactionOverrides(ctx, teamID, channelID)
	if err != nil {
		return "", err
	}
	overrides := channel
	if workspace {
		overrides = ws
	}
	updated := domain.TierOverrides{}
	for t, existing := range overrides {
		kept := existing[:0:0]
		for _, e := range existing {
			if !contains(emojis, e) {
				kept = append(kept, e)
			}
		}
		if len(kept) > 0 {
			updated[t] = kept
		}
	}
	updated[tier] = emojis

	if err := s.store.SetReactionOverrides(ctx, teamID, channelID, workspace, actorUserID, updated); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s now %s %d karma in %s.", strings.Join(colons(emojis), " "), verb(tier), abs(tier), scopeName(workspace)), nil
}

func (s *ReactionSettingsService) Reset(ctx context.Context, teamID, channelID string, workspace bool, actorUserID string) (string, error) {
	if err := s.store.SetReactionOverrides(ctx, teamID, channelID, workspace, actorUserID, domain.TierOverrides{}); err != nil {
		return "", err
	}
	if workspace {
		return "Karma reactions reset to the defaults for this workspace. Channels with their own emojis keep them.", nil
	}
	return "Karma reactions in this channel now follow the workspace settings.", nil
}

func verb(tier int) string {
	if tier < 0 {
		return "takes"
	}
	return "gives"
}

func scopeName(workspace bool) string {
	if workspace {
		return "this workspace"
	}
	return "this channel"
}

func colons(emojis []string) []string {
	out := make([]string, len(emojis))
	for i, e := range emojis {
		out[i] = ":" + e + ":"
	}
	return out
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
