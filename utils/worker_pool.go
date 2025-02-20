package utils

import (
	"context"
	"sync"
)

type WorkerPool struct {
	ctx     context.Context
	workers chan struct{}
	wg      sync.WaitGroup
}

func NewWorkerPool(ctx context.Context, size int) *WorkerPool {
	return &WorkerPool{
		ctx:     ctx,
		workers: make(chan struct{}, size),
		wg:      sync.WaitGroup{},
	}
}

func (p *WorkerPool) Run(task func()) {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()

		select {
		case p.workers <- struct{}{}:
			defer func() { <-p.workers }()

			// Check context before starting
			if p.ctx.Err() != nil {
				return
			}

			task()
		case <-p.ctx.Done():
			return
		}
	}()
}

func (p *WorkerPool) WaitAndClose() {
	p.wg.Wait()
	close(p.workers)
}
