package notifier

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

type MultiNotifier struct {
	notifiers []Notifier
}

type NotifierDependencies struct {
	TgToken        string
	TgChatID       string
	SlackWebHook   string
	DiscordWebHook string
}

func InitMulti(cfg *NotifierDependencies, channels []string) (Notifier, error) {
	sharedClient := http.Client{
		Timeout: 5 * time.Second,
	}

	var list []Notifier

	for _, ch := range channels {
		switch ch {
		case "telegram":
			tg := newTelegram(cfg.TgToken, cfg.TgChatID, &sharedClient)
			list = append(list, tg)
		case "slack":
			sl := newSlack(&sharedClient, cfg.SlackWebHook)
			list = append(list, sl)
		case "discord":
			ds := newDiscord(&sharedClient, cfg.DiscordWebHook)
			list = append(list, ds)
		default:
			slog.Warn("unknown channel", "channel", ch)
		}
	}

	if len(list) == 0 {
		return nil, errors.New("no valid notification channels initialized")
	}

	return newMulti(list...), nil
}

func newMulti(notifiers ...Notifier) Notifier {
	return &MultiNotifier{
		notifiers: notifiers,
	}
}

func (m *MultiNotifier) SendAlert(ctx context.Context, msg string) error {
	var (
		errs []error
		mu   sync.Mutex
		wg   sync.WaitGroup
	)
	errs = make([]error, 0, 3)

	for _, v := range m.notifiers {
		wg.Add(1)
		go func(target Notifier) {
			defer wg.Done()

			if err := target.SendAlert(ctx, msg); err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		}(v)
	}

	wg.Wait()

	if len(errs) > 0 {
		return fmt.Errorf("failed to dispatch to all targets: %w", errors.Join(errs...))
	}

	return nil
}
