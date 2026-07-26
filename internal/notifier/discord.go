package notifier

import (
	"context"
	"fmt"
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

	alertCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	err := doRequest(alertCtx, d.dsClient, d.webHook, params)

	if err != nil {
		return fmt.Errorf("failed to send an alert to discord: %w", err)
	}

	return nil
}
