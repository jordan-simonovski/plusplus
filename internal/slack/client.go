package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	chatPostMessageURL     = "https://slack.com/api/chat.postMessage"
	usergroupsUsersListURL = "https://slack.com/api/usergroups.users.list"
)

type APIClient struct {
	token      string
	httpClient *http.Client
}

func NewAPIClient(token string) *APIClient {
	return &APIClient{
		token: token,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type postMessageRequest struct {
	Channel  string `json:"channel"`
	Text     string `json:"text"`
	ThreadTS string `json:"thread_ts,omitempty"`
}

type postMessageResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error"`
}

func (c *APIClient) PostMessage(ctx context.Context, channelID string, text string, threadTS string) error {
	payload, err := json.Marshal(postMessageRequest{
		Channel:  channelID,
		Text:     text,
		ThreadTS: threadTS,
	})
	if err != nil {
		return fmt.Errorf("encode slack post payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, chatPostMessageURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create slack request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send slack request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read slack response: %w", err)
	}

	var out postMessageResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return fmt.Errorf("decode slack response: %w", err)
	}

	if !out.OK {
		return fmt.Errorf("slack chat.postMessage failed: %s", out.Error)
	}

	return nil
}

type usergroupsUsersListResponse struct {
	OK    bool     `json:"ok"`
	Users []string `json:"users"`
	Error string   `json:"error"`
}

// ListUserGroupMembers calls usergroups.users.list (requires usergroups:read).
func (c *APIClient) ListUserGroupMembers(ctx context.Context, teamID, userGroupID string) ([]string, error) {
	form := url.Values{}
	form.Set("usergroup", userGroupID)
	if teamID != "" {
		form.Set("team_id", teamID)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, usergroupsUsersListURL, bytes.NewReader([]byte(form.Encode())))
	if err != nil {
		return nil, fmt.Errorf("create slack usergroups request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send slack usergroups request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read slack usergroups response: %w", err)
	}

	var out usergroupsUsersListResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("decode slack usergroups response: %w", err)
	}

	if !out.OK {
		return nil, fmt.Errorf("slack usergroups.users.list failed: %s", out.Error)
	}

	return out.Users, nil
}
