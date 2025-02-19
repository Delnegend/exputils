package tasks

import (
	"context"
	"fmt"
	"time"
)

func ExampleTask(ctx context.Context, setProgressBase func(func() float64) func(), sendWarning func(error)) {
	setProgressBase(func() float64 { return 1.0 / 3.0 })()

	for i := 1; i < 4; i++ {
		select {
		case <-ctx.Done():
			sendWarning(ctx.Err())
			return
		case <-time.After(time.Second):
			sendWarning(fmt.Errorf("example warning %d", i))
		}

		setProgressBase(func() float64 {
			return float64(i) / 3
		})()
	}
}
