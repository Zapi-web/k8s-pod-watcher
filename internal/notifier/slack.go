package notifier

import (
	"context"
	"fmt"
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

	alertCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	err := doRequest(alertCtx, s.slClient, s.webHook, params)

	if err != nil {
		return fmt.Errorf("failed to send an alert to slack: %w", err)
	}

	return nil
}
