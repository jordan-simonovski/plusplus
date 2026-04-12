package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const chatPostMessageURL = "https://slack.com/api/chat.postMessage"

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
