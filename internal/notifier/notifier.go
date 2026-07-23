package notifier

import "context"

type Notifier interface {
	SendAlert(ctx context.Context, alert string) error
}
