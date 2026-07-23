package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

type SlackNotifier struct {
	webHook  string
	slClient *http.Client
}

type slSendParams struct {
	Content string `json:"text"`
}

func newSlack(client *http.Client, webHook string) *SlackNotifier {
	if client == nil {
		client = &http.Client{
			Timeout: 5 * time.Second,
		}
	}

	return &SlackNotifier{
		webHook:  webHook,
		slClient: client,
	}
}

func (s *SlackNotifier) SendAlert(ctx context.Context, alert string) error {
	params := slSendParams{
		Content: alert,
	}

	body, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("failed to marshal alert params: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.webHook, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create http request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.slClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send slack request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Error("failed to close response body", "err", err)
		}
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("slack API error (statud %d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}
