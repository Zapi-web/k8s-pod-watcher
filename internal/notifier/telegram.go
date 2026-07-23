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
		ParseMode: "Markdown",
	}

	body, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("failed to marshal alert params: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create http request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.tgClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send telegram request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Error("failed to close response body", "err", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram returned non-200 status (statud %d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}
