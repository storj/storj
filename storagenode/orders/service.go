// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package orders

import (
	"context"
	"errors"
	"io"
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
	orders      DB
	trust       *trust.Pool

	Sender  *sync2.Cycle
	Cleanup *sync2.Cycle
}

// NewService creates an order service.
func NewService(log *zap.Logger, dialer rpc.Dialer, ordersStore *FileStore, orders DB, trust *trust.Pool, config Config) *Service {
	return &Service{
		log:         log,
		dialer:      dialer,
		ordersStore: ordersStore,
		orders:      orders,
		config:      config,
		trust:       trust,

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

	deleted, err := service.orders.CleanArchive(ctx, deleteBefore)
	if err != nil {
		service.log.Error("cleaning DB archive", zap.Error(err))
		return nil
	}

	err = service.ordersStore.CleanArchive(deleteBefore)
	if err != nil {
		service.log.Error("cleaning filestore archive", zap.Error(err))
		return nil
	}

	service.log.Debug("cleanup finished", zap.Int("items deleted", deleted))
	return nil
}

// SendOrders sends the orders using now as the current time.
func (service *Service) SendOrders(ctx context.Context, now time.Time) {
	defer mon.Task()(&ctx)(nil)
	service.log.Debug("sending")

	// If there are orders in the database, send from there.
	// Otherwise, send from the filestore.
	service.sendOrdersFromDB(ctx)
	service.sendOrdersFromFileStore(ctx, now)
}

func (service *Service) sendOrdersFromDB(ctx context.Context) (hasOrders bool) {
	defer mon.Task()(&ctx)(nil)

	const batchSize = 1000
	hasOrders = true

	ordersBySatellite, err := service.orders.ListUnsentBySatellite(ctx)
	if err != nil {
		if ordersBySatellite == nil {
			service.log.Error("listing orders", zap.Error(err))
			hasOrders = false
			return hasOrders
		}

		service.log.Warn("DB contains invalid marshalled orders", zap.Error(err))
	}

	requests := make(chan ArchiveRequest, batchSize)
	var batchGroup errgroup.Group
	batchGroup.Go(func() error { return service.handleBatches(ctx, requests) })

	if len(ordersBySatellite) > 0 {
		var group errgroup.Group
		ctx, cancel := context.WithTimeout(ctx, service.config.SenderTimeout)
		defer cancel()

		for satelliteID, orders := range ordersBySatellite {
			satelliteID, orders := satelliteID, orders
			group.Go(func() error {
				service.Settle(ctx, satelliteID, orders, requests)
				return nil
			})
		}

		_ = group.Wait() // doesn't return errors
	} else {
		service.log.Debug("no orders to send")
		hasOrders = false
	}

	close(requests)
	err = batchGroup.Wait()
	if err != nil {
		service.log.Error("archiving orders", zap.Error(err))
	}
	return hasOrders
}

// Settle uploads orders to the satellite.
func (service *Service) Settle(ctx context.Context, satelliteID storj.NodeID, orders []*ordersfile.Info, requests chan ArchiveRequest) {
	log := service.log.Named(satelliteID.String())
	err := service.settle(ctx, log, satelliteID, orders, requests)
	if err != nil {
		log.Error("failed to settle orders", zap.Error(err))
	}
}

func (service *Service) handleBatches(ctx context.Context, requests chan ArchiveRequest) (err error) {
	defer mon.Task()(&ctx)(&err)

	// In case anything goes wrong, discard everything from the channel.
	defer func() {
		for range requests {
		}
	}()

	buffer := make([]ArchiveRequest, 0, cap(requests))

	archive := func(ctx context.Context, archivedAt time.Time, requests ...ArchiveRequest) error {
		if err := service.orders.Archive(ctx, time.Now().UTC(), buffer...); err != nil {
			if !OrderNotFoundError.Has(err) {
				return err
			}

			service.log.Warn("some unsent order aren't in the DB", zap.Error(err))
		}

		return nil
	}

	for request := range requests {
		buffer = append(buffer, request)
		if len(buffer) < cap(buffer) {
			continue
		}

		if err := archive(ctx, time.Now().UTC(), buffer...); err != nil {
			return err
		}
		buffer = buffer[:0]
	}

	if len(buffer) > 0 {
		return archive(ctx, time.Now().UTC(), buffer...)
	}

	return nil
}

func (service *Service) settle(ctx context.Context, log *zap.Logger, satelliteID storj.NodeID, orders []*ordersfile.Info, requests chan ArchiveRequest) (err error) {
	defer mon.Task()(&ctx)(&err)

	log.Info("sending", zap.Int("count", len(orders)))
	defer log.Info("finished")

	nodeurl, err := service.trust.GetNodeURL(ctx, satelliteID)
	if err != nil {
		return OrderError.New("unable to get satellite address: %w", err)
	}

	conn, err := service.dialer.DialNodeURL(ctx, nodeurl)
	if err != nil {
		return OrderError.New("unable to connect to the satellite: %w", err)
	}
	defer func() { err = errs.Combine(err, conn.Close()) }()

	stream, err := pb.NewDRPCOrdersClient(conn).Settlement(ctx)
	if err != nil {
		return OrderError.New("failed to start settlement: %w", err)
	}

	var group errgroup.Group
	var sendErrors errs.Group

	group.Go(func() error {
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
				sendErrors.Add(err)
				return nil
			}
		}

		err := stream.CloseSend()
		if err != nil {
			err = OrderError.New("CloseSend settlement agreements returned an error: %w", err)
			log.Error("rpc client error when closing sender ", zap.Error(err))
			sendErrors.Add(err)
		}

		return nil
	})

	var errList errs.Group
	for {
		response, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			err = OrderError.New("failed to receive settlement response: %w", err)
			log.Error("rpc client error when receiving new order settlements", zap.Error(err))
			errList.Add(err)
			break
		}

		var status Status
		switch response.Status {
		case pb.SettlementResponse_ACCEPTED:
			status = StatusAccepted
		case pb.SettlementResponse_REJECTED:
			status = StatusRejected
		default:
			err := OrderError.New("unexpected settlement status response: %d", response.Status)
			log.Error("rpc client received an unexpected new orders settlement status",
				zap.Error(err), zap.Any("response", response),
			)
			errList.Add(err)
			continue
		}

		requests <- ArchiveRequest{
			Satellite: satelliteID,
			Serial:    response.SerialNumber,
			Status:    status,
		}
	}

	// errors of this group are reported to sendErrors and it always return nil
	_ = group.Wait()
	errList.Add(sendErrors...)

	return errList.Err()
}

func (service *Service) sendOrdersFromFileStore(ctx context.Context, now time.Time) {
	defer mon.Task()(&ctx)(nil)

	errorSatellites := make(map[storj.NodeID]struct{})
	var errorSatellitesMu sync.Mutex

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
				log := service.log.Named(satelliteID.String())
				status, err := service.settleWindow(ctx, log, satelliteID, unsentInfo.InfoList)
				if err != nil {
					// satellite returned an error, but settlement was not explicitly rejected; we want to retry later
					errorSatellitesMu.Lock()
					errorSatellites[satelliteID] = struct{}{}
					errorSatellitesMu.Unlock()
					log.Error("failed to settle orders for satellite", zap.String("satellite ID", satelliteID.String()), zap.Error(err))
					return nil
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

func (service *Service) settleWindow(ctx context.Context, log *zap.Logger, satelliteID storj.NodeID, orders []*ordersfile.Info) (status pb.SettlementWithWindowResponse_Status, err error) {
	defer mon.Task()(&ctx)(&err)

	log.Info("sending", zap.Int("count", len(orders)))
	defer log.Info("finished")

	nodeurl, err := service.trust.GetNodeURL(ctx, satelliteID)
	if err != nil {
		return 0, OrderError.New("unable to get satellite address: %w", err)
	}

	conn, err := service.dialer.DialNodeURL(ctx, nodeurl)
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
