package notifier

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/go-telegram/bot"
)

type TelegramNotifier struct {
	tgBot *bot.Bot
}

func New(token string) (*TelegramNotifier, error) {
	b, err := bot.New(token)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to bot via token: %w", err)
	}

	return &TelegramNotifier{
		tgBot: b,
	}, nil
}

func (t *TelegramNotifier) Start(ctx context.Context) {
	slog.Info("trying to start the bot...")
	t.tgBot.Start(ctx)
}

func (t *TelegramNotifier) SendAlert(ctx context.Context, chatID string, alert string) error {
	params := &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      alert,
		ParseMode: "Markdown",
	}

	_, err := t.tgBot.SendMessage(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to send an alert: %w", err)
	}

	return nil
}

func (t *TelegramNotifier) Stop(ctx context.Context) {
	slog.Info("trying to stop the bot...")
	_, _ = t.tgBot.Close(ctx)
}
