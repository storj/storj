package testqueue

import (
	"context"
	"golang.org/x/sync/errgroup"
	"sync"

	"go.uber.org/zap"

	"storj.io/storj/storage"
)

type workFunc func(Work) error

type WorkGroup struct {
	ctx     context.Context
	cancel  context.CancelFunc
	log     *zap.Logger
	queue   storage.Queue
	workers int
	workF   workFunc

	group errgroup.Group

	cond    sync.Cond
	allDone *bool
}

type Work struct {
	Ctx    context.Context
	Cancel context.CancelFunc
	Log    *zap.Logger
	Item   storage.Value
	Queue  storage.Queue
}

func NewWorkGroup(ctx context.Context, log *zap.Logger, workers int, workF workFunc) *WorkGroup {
	groupCtx, cancelGroup := context.WithCancel(ctx)
	return &WorkGroup{
		ctx:     groupCtx,
		cancel:  cancelGroup,
		log:     log,
		workers: workers,
		workF:   workF,
	}
}

func (wg *WorkGroup) Go() {
	for i := 0; i < wg.workers; i++ {
		wg.group.Go(func() error {
			wg.cond.L.Lock()
			for {
				select {
				case <-wg.ctx.Done():
					return nil
				default:
					item, err := wg.queue.Dequeue()
					switch {
					case storage.ErrEmptyQueue.Has(err):
						wg.cond.Wait()
					case err != nil:
						return err
					}

					if err := wg.workF(
						Work{
							Ctx:    wg.ctx,
							Cancel: wg.cancel,
							Log:    wg.log,
							Item:   item,
							Queue:  wg.queue,
						},
					); err != nil {
						return err
					}

					wg.cond.Signal()
					wg.cond.L.Unlock()
				}
			}
		})
	}
}

func (wg *WorkGroup) Wait() error {
	return wg.group.Wait()
}
