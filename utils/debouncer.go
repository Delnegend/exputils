package utils

import "time"

func Debouncer(interval time.Duration, fn func()) func() {
	var timer *time.Timer
	return func() {
		if timer != nil {
			timer.Stop()
		}
		timer = time.AfterFunc(interval, fn)
	}
}
