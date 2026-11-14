package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const slackAPIBase = "https://slack.com/api/"

// maxResponseBytes bounds how much of a Slack response is read. Real responses are a few KB.
const maxResponseBytes = 1 << 20

// APIClient calls the Slack Web API with one workspace's bot token.
type APIClient struct {
	token      string
	httpClient *http.Client
}

// newHTTPClient is the client every APIClient shares unless given another.
func newHTTPClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Second}
}

func newAPIClient(token string, httpClient *http.Client) *APIClient {
	return &APIClient{token: token, httpClient: httpClient}
}

// slackStatus is the envelope every Web API response carries.
type slackStatus struct {
	OK    bool   `json:"ok"`
	Error string `json:"error"`
}

// call posts body to the Slack Web API method (for example "chat.postMessage") and decodes
// the response into out (nil to ignore it). Errors name the method so operators can tell
// calls apart in logs: an HTTP failure gives the status (and Retry-After on 429), and a
// Slack-level failure gives Slack's error code as "slack <method> failed: <code>".
func (c *APIClient) call(ctx context.Context, method, contentType string, body io.Reader, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, slackAPIBase+method, body)
	if err != nil {
		return fmt.Errorf("create slack %s request: %w", method, err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", contentType)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send slack %s request: %w", method, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		if resp.StatusCode == http.StatusTooManyRequests {
			return fmt.Errorf("slack %s: HTTP 429 rate limited, retry after %ss", method, resp.Header.Get("Retry-After"))
		}
		return fmt.Errorf("slack %s: HTTP %d", method, resp.StatusCode)
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return fmt.Errorf("read slack %s response: %w", method, err)
	}
	var status slackStatus
	if err := json.Unmarshal(raw, &status); err != nil {
		return fmt.Errorf("decode slack %s response: %w", method, err)
	}
	if !status.OK {
		return fmt.Errorf("slack %s failed: %s", method, status.Error)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("decode slack %s response: %w", method, err)
	}
	return nil
}

func (c *APIClient) callForm(ctx context.Context, method string, form url.Values, out any) error {
	return c.call(ctx, method, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()), out)
}

type postMessageRequest struct {
	Channel  string `json:"channel"`
	Text     string `json:"text"`
	ThreadTS string `json:"thread_ts,omitempty"`
}

func (c *APIClient) PostMessage(ctx context.Context, channelID string, text string, threadTS string) error {
	payload, err := json.Marshal(postMessageRequest{Channel: channelID, Text: text, ThreadTS: threadTS})
	if err != nil {
		return fmt.Errorf("encode slack post payload: %w", err)
	}
	return c.call(ctx, "chat.postMessage", "application/json; charset=utf-8", bytes.NewReader(payload), nil)
}

// ListUserGroupMembers calls usergroups.users.list (requires usergroups:read).
func (c *APIClient) ListUserGroupMembers(ctx context.Context, teamID, userGroupID string) ([]string, error) {
	form := url.Values{}
	form.Set("usergroup", userGroupID)
	if teamID != "" {
		form.Set("team_id", teamID)
	}
	var out struct {
		Users []string `json:"users"`
	}
	if err := c.callForm(ctx, "usergroups.users.list", form, &out); err != nil {
		return nil, err
	}
	return out.Users, nil
}

type usersInfoResponse struct {
	User struct {
		RealName string `json:"real_name"`
		IsAdmin  bool   `json:"is_admin"`
		IsOwner  bool   `json:"is_owner"`
		Profile  struct {
			DisplayName string `json:"display_name"`
			RealName    string `json:"real_name"`
		} `json:"profile"`
	} `json:"user"`
}

// DisplayName calls users.info (requires users:read) and returns the best
// available human name: display_name, then profile.real_name, then real_name.
func (c *APIClient) DisplayName(ctx context.Context, userID string) (string, error) {
	out, err := c.usersInfo(ctx, userID)
	if err != nil {
		return "", err
	}
	if name := firstNonEmpty(out.User.Profile.DisplayName, out.User.Profile.RealName, out.User.RealName); name != "" {
		return name, nil
	}
	return "", fmt.Errorf("slack users.info returned no name for %s", userID)
}

// IsWorkspaceAdmin reports whether the user is a workspace admin or owner (users:read).
func (c *APIClient) IsWorkspaceAdmin(ctx context.Context, userID string) (bool, error) {
	out, err := c.usersInfo(ctx, userID)
	if err != nil {
		return false, err
	}
	return out.User.IsAdmin || out.User.IsOwner, nil
}

func (c *APIClient) usersInfo(ctx context.Context, userID string) (usersInfoResponse, error) {
	form := url.Values{}
	form.Set("user", userID)
	var out usersInfoResponse
	err := c.callForm(ctx, "users.info", form, &out)
	return out, err
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
