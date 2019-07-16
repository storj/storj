// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"

	"storj.io/storj/internal/sync2"
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

// LoopConfig contains configurable values for the metainfo loop
type LoopConfig struct {
	CoalesceDuration time.Duration `help:"how frequently metainfoloop should iterate over segments" releaseDefault:"30s" devDefault:"0h0m10s"`
}

// LoopService is a metainfo loop service
type LoopService struct {
	waitingObservers  []Observer
	observers         []Observer
	Loop              *sync2.Cycle
	metainfo          *Service
	observersCombined chan bool
	loopEnded         chan error
}

// NewLoop creates a new metainfo loop service
func NewLoop(config LoopConfig, metainfo *Service) *LoopService {
	return &LoopService{
		Loop:              sync2.NewCycle(config.CoalesceDuration),
		metainfo:          metainfo,
		observersCombined: make(chan bool),
		loopEnded:         make(chan error),
	}
}

// Run starts the looping service.
func (service *LoopService) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	return service.Loop.Run(ctx, func(ctx context.Context) error {
		if len(service.waitingObservers) == 0 {
			return nil
		}
		// wait for observers
		// coalesce incoming observers, within 5s
		// TODO fix concurrent read/writes or something
		service.observers = service.waitingObservers
		service.waitingObservers = []Observer{}

		service.observersCombined <- true

		err = service.metainfo.Iterate(ctx, "", "", true, false,
			func(ctx context.Context, it storage.Iterator) error {
				var item storage.ListItem

				// iterate over every segment in metainfo
				for it.Next(ctx, &item) {
					pointer := &pb.Pointer{}

					err = proto.Unmarshal(item.Value, pointer)
					if err != nil {
						return LoopError.New("error unmarshalling pointer %s", err)
					}

					path := storj.Path(item.Key.String())

					// send segment info to every observer
					for _, o := range service.observers {
						remote := pointer.GetRemote()
						if remote != nil {
							_ = o.RemoteSegment(ctx, path, pointer)

							pathElements := storj.SplitPath(path)
							if len(pathElements) >= 2 && pathElements[1] == "l" {
								_ = o.RemoteObject(ctx, path, pointer)
							}

						} else {
							_ = o.InlineSegment(ctx, path, pointer)
						}
					}
				}
				return nil
			})

		service.loopEnded <- err

		return err
	})
}

// Close halts the metainfo loop
func (service *LoopService) Close() error {
	service.Loop.Close()
	return nil
}

// Join will join the looper for one full cycle until completion and then returns.
// On ctx cancel the observer will return without completely finishing.
// Only on full complete iteration it will return nil.
func (service *LoopService) Join(ctx context.Context, observer Observer) (err error) {
	// TODO fix concurrent read/writes or something
	service.waitingObservers = append(service.waitingObservers, observer)

	// wait for observer combine
	<-service.observersCombined

	// wait for loop to iterate over all segments
	err = <-service.loopEnded

	return err
}
