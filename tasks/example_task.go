package tasks

import (
	"context"
	"time"
)

func ExampleTask(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(5 * time.Second):
		return nil
	}
}
