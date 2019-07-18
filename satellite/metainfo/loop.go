// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"
	"fmt"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
)

var (
	// LoopError is a standard error class for this component.
	LoopError = errs.Class("metainfo loop error")
	// LoopClosedError is a loop closed error
	LoopClosedError = LoopError.New("loop closed")
)

// Observer is an interface defining an observer that can subscribe to the metainfo loop.
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
		observer.Finish()
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

// LoopConfig contains configurable values for the metainfo loop.
type LoopConfig struct {
	CoalesceDuration time.Duration `help:"how long to wait for new observers before starting iteration" releaseDefault:"5s" devDefault:"5s"`
}

// Loop is a metainfo loop service.
type Loop struct {
	config   LoopConfig
	metainfo *Service
	join     chan *observerContext
	done     chan struct{}
	cancel   func()
}

// NewLoop creates a new metainfo loop service.
func NewLoop(config LoopConfig, metainfo *Service) *Loop {
	return &Loop{
		metainfo: metainfo,
		config:   config,
		join:     make(chan *observerContext),
		done:     make(chan struct{}),
	}
}

// Join will join the looper for one full cycle until completion and then returns.
// On ctx cancel the observer will return without completely finishing.
// Only on full complete iteration it will return nil.
func (loop *Loop) Join(ctx context.Context, observer Observer) (err error) {
	defer mon.Task()(&ctx)(&err)

	obsContext := &observerContext{
		Observer: observer,
		ctx:      ctx,
		done:     make(chan error),
	}

	select {
	case loop.join <- obsContext:
	case <-ctx.Done():
		return ctx.Err()
	case <-loop.done:
		return LoopClosedError
	}

	return obsContext.Wait()
}

// Run starts the looping service.
func (loop *Loop) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	ctx, cancel := context.WithCancel(ctx)
	loop.cancel = cancel
	defer close(loop.done)

	for {
		err := loop.runOnce(ctx)
		if err != nil {
			return err
		}
	}
}

// runOnce goes through metainfo one time and sends information to observers
func (loop *Loop) runOnce(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	var observers []*observerContext

	defer func() {
		for _, observer := range observers {
			observer.Finish()
		}
	}()

	// wait for the first observer, or exit because context is canceled
	select {
	case observer := <-loop.join:
		observers = append(observers, observer)
	case <-ctx.Done():
		return ctx.Err()
	}

	// after the first observer is found, set timer for CoalesceDuration and add any observers that try to join before the timer is up
	timer := time.NewTimer(loop.config.CoalesceDuration)
waitformore:
	for {
		select {
		case observer := <-loop.join:
			observers = append(observers, observer)
		case <-timer.C:
			break waitformore
		case <-ctx.Done():
			for _, observer := range observers {
				observer.HandleError(ctx.Err())
			}
			observers = nil
			return ctx.Err()
		}
	}

	err = loop.metainfo.Iterate(ctx, "", "", true, false,
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
				isLastSeg := len(pathElements) >= 2 && pathElements[1] == "l"

				nextObservers := observers[:0]

				fmt.Println(observers)
				// send segment info to every observer

				for _, observer := range observers {
					remote := pointer.GetRemote()
					if remote != nil {
						if observer.HandleError(observer.RemoteSegment(ctx, path, pointer)) {
							continue
						}

						if isLastSeg {
							if observer.HandleError(observer.RemoteObject(ctx, path, pointer)) {
								continue
							}
						}
					} else if observer.HandleError(observer.InlineSegment(ctx, path, pointer)) {
						continue
					}

					select {
					case <-observer.ctx.Done():
						observer.HandleError(observer.ctx.Err())
						continue
					default:
					}

					// for the next segment, only iterate over observers that did not have an error or canceled context
					nextObservers = append(nextObservers, observer)
				}

				observers = nextObservers
				if len(observers) == 0 {
					return nil
				}

				// if context has been canceled, send the error to observers and exit. Otherwise, continue
				select {
				case <-ctx.Done():
					fmt.Printf("context is done: %p\n", loop)
					for _, observer := range observers {
						observer.HandleError(ctx.Err())
					}
					observers = nil
					return ctx.Err()
				default:
					fmt.Printf("continuing iteration: %p\n", loop)
				}
			}
			return nil
		})

	if err != nil {
		for _, observer := range observers {
			observer.HandleError(err)
		}
		return err
	}
	return nil
}

// Wait waits for run to be finished.
func (loop *Loop) Wait() {
	<-loop.done
}
