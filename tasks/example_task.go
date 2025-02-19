package tasks

import (
	"context"
	"fmt"
	"time"
)

func ExampleTask(ctx context.Context, setProgressChan chan float64, warnChan chan<- error) {
	for i := 1; i < 4; i++ {
		select {
		case <-ctx.Done():
			go func(cause error) {
				warnChan <- cause
			}(ctx.Err())
			return
		case setProgressChan <- float64(i) / 3:
		}

		select {
		case <-ctx.Done():
			go func(cause error) {
				warnChan <- cause
			}(ctx.Err())
			return
		case <-time.After(time.Second):
			go func(i int) {
				warnChan <- fmt.Errorf("example warning %d", i)
			}(i)
		}
	}
}
