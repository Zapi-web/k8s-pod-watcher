package notifier

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

type BatchNotiifer struct {
	underlying Notifier
	interval   time.Duration
	maxBatch   int
	maxDisplay int
	ch         chan string
	wg         sync.WaitGroup
	cancel     context.CancelFunc
}

func NewBatchNotifier(underlying Notifier, interval time.Duration, maxBatch, maxDisplay int) *BatchNotiifer {
	return &BatchNotiifer{
		underlying: underlying,
		interval:   interval,
		maxBatch:   maxBatch,
		maxDisplay: maxDisplay,
		ch:         make(chan string, 200),
	}
}

func (b *BatchNotiifer) Start(ctx context.Context) {
	ctx, b.cancel = context.WithCancel(ctx)
	b.wg.Add(1)
	go b.run(ctx)
}

func (b *BatchNotiifer) SendAlert(ctx context.Context, msg string) error {
	select {
	case b.ch <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (b *BatchNotiifer) run(ctx context.Context) {
	defer b.wg.Done()

	ticker := time.NewTicker(b.interval)
	defer ticker.Stop()

	buffer := make([]string, 0, 20)

	flush := func() {
		if len(buffer) == 0 {
			return
		}

		var finalMsg string
		if len(buffer) <= b.maxDisplay {
			finalMsg = strings.Join(buffer, "\n\n")
		} else {
			displayed := buffer[:b.maxDisplay]
			remaining := len(buffer) - b.maxDisplay

			finalMsg = fmt.Sprintf(
				"**Alert Storm Detected** (%d pod failures):\n\n%s\n\n... and %d other pods also crashed",
				len(buffer),
				strings.Join(displayed, "\n\n"),
				remaining,
			)
		}

		err := b.underlying.SendAlert(ctx, finalMsg)

		if err != nil {
			slog.Error("failed to send alerts from butch", "err", err)
		}

		buffer = buffer[:0]
	}

	for {
		select {
		case <-ctx.Done():
			flush()
			return
		case msg := <-b.ch:
			buffer = append(buffer, msg)
			if len(buffer) >= b.maxBatch {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

func (b *BatchNotiifer) Stop() {
	if b.cancel != nil {
		b.cancel()
	}
	b.wg.Wait()
}
