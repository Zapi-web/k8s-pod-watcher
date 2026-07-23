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

type DiscordNotifier struct {
	webHook  string
	dsClient *http.Client
}

type dsSendParams struct {
	Content string `json:"content"`
}

func newDiscord(client *http.Client, webHook string) *DiscordNotifier {
	if client == nil {
		client = &http.Client{
			Timeout: 5 * time.Second,
		}
	}

	return &DiscordNotifier{
		webHook:  webHook,
		dsClient: client,
	}
}

func (d *DiscordNotifier) SendAlert(ctx context.Context, alert string) error {
	params := dsSendParams{
		Content: alert,
	}

	body, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("failed to marshal alert params: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.webHook, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create http request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.dsClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send discord request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Error("failed to close response body", "err", err)
		}
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("discord API error (statud %d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}
