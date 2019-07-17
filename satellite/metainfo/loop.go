// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
)

var (
	// LoopError is a standard error class for this component
	LoopError = errs.Class("metainfo loop error")
)

// Observer is an interface defining an observer that can subscribe to the metainfo loop
type Observer interface {
	RemoteSegment(context.Context, storj.Path, *pb.Pointer) error
	RemoteObject(context.Context, storj.Path, *pb.Pointer) error
	InlineSegment(context.Context, storj.Path, *pb.Pointer) error
}

type observerContext struct {
	Observer
	ctx  context.Context
	done chan error
}

func (observer *observerContext) HandleError(err error) bool {
	if err != nil {
		observer.done <- err
		close(observer.done)
		return true
	}
	return false
}

func (observer *observerContext) Finish() {
	close(observer.done)
}

func (observer *observerContext) Wait() error {
	return <-observer.done
}

// LoopConfig contains configurable values for the metainfo loop
type LoopConfig struct {
	CoalesceDuration time.Duration `help:"how long to wait for new observers before starting iteration" releaseDefault:"5s" devDefault:"5s"`
}

// Loop is a metainfo loop service
type Loop struct {
	config   LoopConfig
	metainfo *Service
	join     chan *observerContext
}

// NewLoop creates a new metainfo loop service
func NewLoop(config LoopConfig, metainfo *Service) *Loop {
	return &Loop{
		metainfo: metainfo,
		config:   config,
		join:     make(chan *observerContext),
	}
}

// Run starts the looping service
func (service *Loop) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	for {
		err := service.runOnce(ctx)
		if err != nil {
			return err
		}
	}
}

// Join will join the looper for one full cycle until completion and then returns.
// On ctx cancel the observer will return without completely finishing.
// Only on full complete iteration it will return nil.
func (service *Loop) Join(ctx context.Context, observer Observer) (err error) {
	defer mon.Task()(&ctx)(&err)

	context := &observerContext{
		Observer: observer,
		ctx:      ctx,
		done:     make(chan error),
	}

	service.join <- context
	err = context.Wait()

	return err
}

func (service *Loop) runOnce(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	var observers []*observerContext

	defer func() {
		for _, observer := range observers {
			observer.Finish()
		}
	}()

	select {
	case observer := <-service.join:
		observers = append(observers, observer)
	case <-ctx.Done():
		return ctx.Err()
	}

	timer := time.NewTimer(service.config.CoalesceDuration)
waitformore:
	for {
		select {
		case observer := <-service.join:
			observers = append(observers, observer)
		case <-timer.C:
			break waitformore
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return service.metainfo.Iterate(ctx, "", "", true, false,
		func(ctx context.Context, it storage.Iterator) error {
			var item storage.ListItem

			// iterate over every segment in metainfo
			for it.Next(ctx, &item) {
				pointer := &pb.Pointer{}

				err = proto.Unmarshal(item.Value, pointer)
				if err != nil {
					// TODO: figure out what to do
					// return LoopError.New("error unmarshalling pointer %s", err)
					continue
				}

				path := item.Key.String()
				pathElements := storj.SplitPath(path)

				nextObservers := observers[:0]

				// send segment info to every observer
				for _, observer := range observers {
					remote := pointer.GetRemote()
					if remote != nil {
						if observer.HandleError(observer.RemoteSegment(ctx, path, pointer)) {
							continue
						}

						if len(pathElements) >= 2 && pathElements[1] == "l" {
							if observer.HandleError(observer.RemoteObject(ctx, path, pointer)) {
								continue
							}
						}
					} else {
						if observer.HandleError(observer.InlineSegment(ctx, path, pointer)) {
							continue
						}
					}

					select {
					case <-observer.ctx.Done():
						observer.HandleError(observer.ctx.Err())
						continue
					default:
					}

					nextObservers = append(nextObservers, observer)
				}

				observers = nextObservers
				if len(observers) == 0 {
					return nil
				}

				select {
				case <-ctx.Done():
					for _, observer := range observers {
						observer.HandleError(ctx.Err())
					}
					observers = nil
					return ctx.Err()
				default:
				}
			}
			return nil
		})
}

// Close halts the metainfo loop
func (service *Loop) Close() error {
	return nil
}
