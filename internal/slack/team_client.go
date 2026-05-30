package slack

import (
	"context"
	"errors"
	"fmt"

	"plusplus/internal/domain"
)

// BotTokenStore resolves a workspace bot token (decrypted from the data store).
type BotTokenStore interface {
	GetBotToken(ctx context.Context, teamID string) (string, error)
}

// TeamResolvingClient uses per-workspace tokens from the store, with optional
// SLACK_BOT_TOKEN fallback when no row exists (single-workspace / dev).
type TeamResolvingClient struct {
	store         BotTokenStore
	fallbackToken string
}

func NewTeamResolvingClient(store BotTokenStore, fallbackToken string) *TeamResolvingClient {
	return &TeamResolvingClient{store: store, fallbackToken: fallbackToken}
}

func (c *TeamResolvingClient) apiClient(ctx context.Context, teamID string) (*APIClient, error) {
	// Multi-tenant mode: the per-workspace token is authoritative. A missing row
	// must fail, not borrow another workspace's (or the global) token.
	if c.store != nil {
		tok, err := c.store.GetBotToken(ctx, teamID)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return nil, fmt.Errorf("workspace %s not installed", teamID)
			}
			return nil, err
		}
		if tok == "" {
			return nil, fmt.Errorf("empty bot token for workspace %s", teamID)
		}
		return NewAPIClient(tok), nil
	}

	// Single-workspace / dev: no store, fall back to the static bot token.
	if c.fallbackToken != "" {
		return NewAPIClient(c.fallbackToken), nil
	}
	return nil, fmt.Errorf("no bot token for workspace %s", teamID)
}

// PostMessage implements WebClient.
func (c *TeamResolvingClient) PostMessage(ctx context.Context, teamID, channelID, text, threadTS string) error {
	client, err := c.apiClient(ctx, teamID)
	if err != nil {
		return err
	}
	return client.PostMessage(ctx, channelID, text, threadTS)
}

// ListUserGroupMembers implements UserGroupMembersLister.
func (c *TeamResolvingClient) ListUserGroupMembers(ctx context.Context, teamID, userGroupID string) ([]string, error) {
	client, err := c.apiClient(ctx, teamID)
	if err != nil {
		return nil, err
	}
	return client.ListUserGroupMembers(ctx, teamID, userGroupID)
}

// DisplayName implements domain.NameResolver.
func (c *TeamResolvingClient) DisplayName(ctx context.Context, teamID, userID string) (string, error) {
	client, err := c.apiClient(ctx, teamID)
	if err != nil {
		return "", err
	}
	return client.DisplayName(ctx, userID)
}
