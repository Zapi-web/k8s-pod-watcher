package notifier

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

type TelegramNotifier struct {
	chatID   string
	url      string
	tgClient *http.Client
}

type tgSendParams struct {
	ChatID    string `json:"chat_id"`
	ParseMode string `json:"parse_mode"`
	Text      string `json:"text"`
}

func newTelegram(token, chatID string, client *http.Client) *TelegramNotifier {
	if client == nil {
		client = &http.Client{
			Timeout: 5 * time.Second,
		}
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)

	return &TelegramNotifier{
		chatID:   chatID,
		url:      url,
		tgClient: client,
	}
}

func (t *TelegramNotifier) SendAlert(ctx context.Context, alert string) error {
	params := tgSendParams{
		ChatID:    t.chatID,
		Text:      alert,
		ParseMode: "MarkdownV2",
	}

	alertCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	err := doRequest(alertCtx, t.tgClient, t.url, params)

	if err != nil {
		return fmt.Errorf("failed to send an alert to telegram: %w", err)
	}

	return nil
}
