package metainfoloop

import (
	"context"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/sync2"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/storage"
)

var (
	// Error is a standard error class for this package
	Error = errs.Class("metainfo loop error")
	mon   = monkit.Package()
)

// Observer is an interface defining an observer that can subscribe to the metainfo loop
type Observer interface {
	RemoteSegment(context.Context, storj.Path, *pb.Pointer) error
	RemoteObject(context.Context, storj.Path, *pb.Pointer) error
	InlineSegment(context.Context, storj.Path, *pb.Pointer) error
}

// Config contains configurable values for the metainfo loop
type Config struct {
	CoalesceDuration time.Duration
}

// Service is a metainfo loop service
type Service struct {
	waitingObservers  []Observer
	observers         []Observer
	Loop              *sync2.Cycle
	metainfo          *metainfo.Service
	observersCombined chan bool
	loopEnded         chan error
}

// New creates a new metainfo loop service
func New(config Config, metainfo *metainfo.Service) *Service {
	return &Service{
		Loop:              sync2.NewCycle(config.CoalesceDuration),
		metainfo:          metainfo,
		observersCombined: make(chan bool),
		loopEnded:         make(chan error),
	}
}

// Run starts the looping service.
func (service *Service) Run(ctx context.Context) (err error) {
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

				for it.Next(ctx, &item) {
					pointer := &pb.Pointer{}

					err = proto.Unmarshal(item.Value, pointer)
					if err != nil {
						return Error.New("error unmarshalling pointer %s", err)
					}

					path := storj.Path(item.Key.String())

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

// Join will join the looper for one full cycle until completion and then returns.
// On ctx cancel the observer will return without completely finishing.
// Only on full complete iteration it will return nil.
func (service *Service) Join(ctx context.Context, observer Observer) (err error) {
	// TODO fix concurrent read/writes or something
	service.waitingObservers = append(service.waitingObservers, observer)

	// wait for observer combine
	<-service.observersCombined

	err = <-service.loopEnded

	return nil
}
