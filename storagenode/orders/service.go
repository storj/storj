// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package orders

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/pb"
	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/storj/storagenode/orders/ordersfile"
	"storj.io/storj/storagenode/trust"
)

var (
	// OrderError represents errors with orders.
	OrderError = errs.Class("order")
	// OrderNotFoundError is the error returned when an order is not found.
	OrderNotFoundError = errs.Class("order not found")

	mon = monkit.Package()
)

// ArchivedInfo contains full information about an archived order.
type ArchivedInfo struct {
	Limit *pb.OrderLimit
	Order *pb.Order

	Status     Status
	ArchivedAt time.Time
}

// Status is the archival status of the order.
type Status byte

// Statuses for satellite responses.
const (
	StatusUnsent Status = iota
	StatusAccepted
	StatusRejected
)

// ArchiveRequest defines arguments for archiving a single order.
type ArchiveRequest struct {
	Satellite storj.NodeID
	Serial    storj.SerialNumber
	Status    Status
}

// DB implements storing orders for sending to the satellite.
//
// architecture: Database
type DB interface {
	// Enqueue inserts order to the list of orders needing to be sent to the satellite.
	Enqueue(ctx context.Context, info *ordersfile.Info) error
	// ListUnsent returns orders that haven't been sent yet.
	ListUnsent(ctx context.Context, limit int) ([]*ordersfile.Info, error)
	// ListUnsentBySatellite returns orders that haven't been sent yet grouped by satellite.
	ListUnsentBySatellite(ctx context.Context) (map[storj.NodeID][]*ordersfile.Info, error)

	// Archive marks order as being handled.
	Archive(ctx context.Context, archivedAt time.Time, requests ...ArchiveRequest) error
	// ListArchived returns orders that have been sent.
	ListArchived(ctx context.Context, limit int) ([]*ArchivedInfo, error)
	// CleanArchive deletes all entries older than the before time.
	CleanArchive(ctx context.Context, deleteBefore time.Time) (int, error)
}

// Config defines configuration for sending orders.
type Config struct {
	MaxSleep          time.Duration `help:"maximum duration to wait before trying to send orders" releaseDefault:"30s" devDefault:"1s"`
	SenderInterval    time.Duration `help:"duration between sending" releaseDefault:"1h0m0s" devDefault:"30s"`
	SenderTimeout     time.Duration `help:"timeout for sending" default:"1h0m0s"`
	SenderDialTimeout time.Duration `help:"timeout for dialing satellite during sending orders" default:"1m0s"`
	CleanupInterval   time.Duration `help:"duration between archive cleanups" default:"5m0s"`
	ArchiveTTL        time.Duration `help:"length of time to archive orders before deletion" default:"168h0m0s"` // 7 days
	Path              string        `help:"path to store order limit files in" default:"$CONFDIR/orders"`
}

// Service sends every interval unsent orders to the satellite.
//
// architecture: Chore
type Service struct {
	log    *zap.Logger
	config Config

	dialer      rpc.Dialer
	ordersStore *FileStore

	trustSource trust.TrustedSatelliteSource

	Sender  *sync2.Cycle
	Cleanup *sync2.Cycle
}

// NewService creates an order service.
func NewService(log *zap.Logger, dialer rpc.Dialer, ordersStore *FileStore, trustSource trust.TrustedSatelliteSource, config Config) *Service {
	return &Service{
		log:         log,
		dialer:      dialer,
		ordersStore: ordersStore,
		config:      config,
		trustSource: trustSource,

		Sender:  sync2.NewCycle(config.SenderInterval),
		Cleanup: sync2.NewCycle(config.CleanupInterval),
	}
}

// Run sends orders on every interval to the appropriate satellites.
func (service *Service) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	var group errgroup.Group

	service.Sender.Start(ctx, &group, func(ctx context.Context) error {
		if err := service.sleep(ctx); err != nil {
			return err
		}

		service.SendOrders(ctx, time.Now())

		return nil
	})
	service.Cleanup.Start(ctx, &group, func(ctx context.Context) error {
		if err := service.sleep(ctx); err != nil {
			return err
		}

		err := service.CleanArchive(ctx, time.Now().Add(-service.config.ArchiveTTL))
		if err != nil {
			service.log.Error("clean archive failed", zap.Error(err))
		}

		return nil
	})

	return group.Wait()
}

// CleanArchive removes all archived orders that were archived before the deleteBefore time.
func (service *Service) CleanArchive(ctx context.Context, deleteBefore time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)
	service.log.Debug("cleaning")

	err = service.ordersStore.CleanArchive(deleteBefore)
	if err != nil {
		service.log.Error("cleaning filestore archive", zap.Error(err))
		return nil
	}

	service.log.Debug("cleanup finished")
	return nil
}

// SendOrders sends the orders using now as the current time.
func (service *Service) SendOrders(ctx context.Context, now time.Time) {
	defer mon.Task()(&ctx)(nil)
	service.log.Debug("sending")

	errorSatellites := make(map[storj.NodeID]struct{})
	var errorSatellitesMu sync.Mutex

	addErrorSatellite := func(satelliteID storj.NodeID) {
		errorSatellitesMu.Lock()
		defer errorSatellitesMu.Unlock()
		errorSatellites[satelliteID] = struct{}{}
	}

	// Continue sending until there are no more windows to send, or all relevant satellites are offline.
	for {
		ordersBySatellite, err := service.ordersStore.ListUnsentBySatellite(ctx, now)
		if err != nil {
			service.log.Error("listing orders", zap.Error(err))
		}
		if len(ordersBySatellite) == 0 {
			service.log.Debug("no orders to send")
			break
		}

		var group errgroup.Group
		attemptedSatellites := 0
		ctx, cancel := context.WithTimeout(ctx, service.config.SenderTimeout)

		for satelliteID, unsentInfo := range ordersBySatellite {
			satelliteID, unsentInfo := satelliteID, unsentInfo
			if _, ok := errorSatellites[satelliteID]; ok {
				continue
			}
			attemptedSatellites++

			group.Go(func() error {
				log := service.log.With(zap.Stringer("satelliteID", satelliteID))

				skipSettlement := false
				nodeURL, err := service.trustSource.GetNodeURL(ctx, satelliteID)
				if err != nil {
					log.Error("unable to get satellite address", zap.Error(err))

					if !errs.Is(err, trust.ErrUntrusted) {
						addErrorSatellite(satelliteID)
						return nil
					}
					skipSettlement = true
				}

				status := pb.SettlementWithWindowResponse_REJECTED
				if !skipSettlement {
					status, err = service.settleWindow(ctx, log, nodeURL, unsentInfo.InfoList)
					if err != nil {
						// satellite returned an error, but settlement was not explicitly rejected; we want to retry later
						addErrorSatellite(satelliteID)
						log.Error("failed to settle orders for satellite", zap.String("satellite ID", satelliteID.String()), zap.Error(err))
						return nil
					}
				} else {
					log.Warn("skipping order settlement for untrusted satellite. Order will be archived", zap.String("satellite ID", satelliteID.String()))
				}

				err = service.ordersStore.Archive(satelliteID, unsentInfo, time.Now().UTC(), status)
				if err != nil {
					log.Error("failed to archive orders", zap.Error(err))
					return nil
				}

				return nil
			})

		}
		_ = group.Wait() // doesn't return errors
		cancel()

		// if all satellites that orders need to be sent to  are offline, exit and try again later.
		if attemptedSatellites == 0 {
			break
		}
	}
}

func (service *Service) settleWindow(ctx context.Context, log *zap.Logger, nodeURL storj.NodeURL, orders []*ordersfile.Info) (status pb.SettlementWithWindowResponse_Status, err error) {
	defer mon.Task()(&ctx)(&err)

	log.Debug("sending", zap.Int("count", len(orders)))
	defer log.Info("finished", zap.Int("count", len(orders)))

	conn, err := service.dialer.DialNodeURL(ctx, nodeURL)
	if err != nil {
		return 0, OrderError.New("unable to connect to the satellite: %w", err)
	}
	defer func() { err = errs.Combine(err, conn.Close()) }()

	stream, err := pb.NewDRPCOrdersClient(conn).SettlementWithWindow(ctx)
	if err != nil {
		return 0, OrderError.New("failed to start settlement: %w", err)
	}

	for _, order := range orders {
		req := pb.SettlementRequest{
			Limit: order.Limit,
			Order: order.Order,
		}
		err := stream.Send(&req)
		if err != nil {
			err = OrderError.New("sending settlement agreements returned an error: %w", err)
			log.Error("rpc client when sending new orders settlements",
				zap.Error(err),
				zap.Any("request", req),
			)
			return 0, err
		}
	}

	res, err := stream.CloseAndRecv()
	if err != nil {
		err = OrderError.New("CloseAndRecv settlement agreements returned an error: %w", err)
		log.Error("rpc client error when closing sender ", zap.Error(err))
		return 0, err
	}

	return res.Status, nil
}

// TestSetLogger sets the logger.
func (service *Service) TestSetLogger(log *zap.Logger) {
	service.log = log
}

// sleep for random interval in [0;maxSleep).
// Returns an error if context was cancelled.
func (service *Service) sleep(ctx context.Context) error {
	if service.config.MaxSleep <= 0 {
		return nil
	}

	jitter := time.Duration(rand.Int63n(int64(service.config.MaxSleep)))
	if !sync2.Sleep(ctx, jitter) {
		return ctx.Err()
	}

	return nil
}

// Close stops the sending service.
func (service *Service) Close() error {
	service.Sender.Close()
	service.Cleanup.Close()
	return nil
}
