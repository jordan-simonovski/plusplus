package slack

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// BotTokenStore resolves a workspace bot token (decrypted from Postgres).
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
	if c.store != nil {
		tok, err := c.store.GetBotToken(ctx, teamID)
		if err == nil && tok != "" {
			return NewAPIClient(tok), nil
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
	}
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
