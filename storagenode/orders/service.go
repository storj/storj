// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package orders

import (
	"context"
	"io"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/sync2"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/storagenode/trust"
)

var (
	// OrderError represents errors with orders
	OrderError = errs.Class("order")
	// OrderNotFoundError is the error returned when an order is not found
	OrderNotFoundError = errs.Class("order not found")

	mon = monkit.Package()
)

// Info contains full information about an order.
type Info struct {
	Limit *pb.OrderLimit
	Order *pb.Order
}

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
type DB interface {
	// Enqueue inserts order to the list of orders needing to be sent to the satellite.
	Enqueue(ctx context.Context, info *Info) error
	// ListUnsent returns orders that haven't been sent yet.
	ListUnsent(ctx context.Context, limit int) ([]*Info, error)
	// ListUnsentBySatellite returns orders that haven't been sent yet grouped by satellite.
	ListUnsentBySatellite(ctx context.Context) (map[storj.NodeID][]*Info, error)

	// Archive marks order as being handled.
	Archive(ctx context.Context, archivedAt time.Time, requests ...ArchiveRequest) error
	// ListArchived returns orders that have been sent.
	ListArchived(ctx context.Context, limit int) ([]*ArchivedInfo, error)
	// CleanArchive deletes all entries older than ttl
	CleanArchive(ctx context.Context, ttl time.Duration) (int, error)
}

// Config defines configuration for sending orders.
type Config struct {
	SenderInterval       time.Duration `help:"duration between sending" default:"1h0m0s"`
	SenderTimeout        time.Duration `help:"timeout for sending" default:"1h0m0s"`
	SenderDialTimeout    time.Duration `help:"timeout for dialing satellite during sending orders" default:"1m0s"`
	SenderRequestTimeout time.Duration `help:"timeout for read/write operations during sending" default:"1h0m0s"`
	CleanupInterval      time.Duration `help:"duration between archive cleanups" default:"24h0m0s"`
	ArchiveTTL           time.Duration `help:"length of time to archive orders before deletion" default:"168h0m0s"` // 7 days
}

// Service sends every interval unsent orders to the satellite.
type Service struct {
	log    *zap.Logger
	config Config

	transport transport.Client
	orders    DB
	trust     *trust.Pool

	Sender  sync2.Cycle
	Cleanup sync2.Cycle
}

// NewService creates an order service.
func NewService(log *zap.Logger, transport transport.Client, orders DB, trust *trust.Pool, config Config) *Service {
	return &Service{
		log:       log,
		transport: transport,
		orders:    orders,
		config:    config,
		trust:     trust,

		Sender:  *sync2.NewCycle(config.SenderInterval),
		Cleanup: *sync2.NewCycle(config.CleanupInterval),
	}
}

// Run sends orders on every interval to the appropriate satellites.
func (service *Service) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	var group errgroup.Group
	service.Sender.Start(ctx, &group, service.sendOrders)
	service.Cleanup.Start(ctx, &group, service.cleanArchive)

	return group.Wait()
}

func (service *Service) cleanArchive(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	service.log.Debug("cleaning")

	deleted, err := service.orders.CleanArchive(ctx, service.config.ArchiveTTL)
	if err != nil {
		service.log.Error("cleaning archive", zap.Error(err))
		return nil
	}

	service.log.Debug("cleanup finished", zap.Int("items deleted", deleted))
	return nil
}

func (service *Service) sendOrders(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	service.log.Debug("sending")

	const batchSize = 1000

	ordersBySatellite, err := service.orders.ListUnsentBySatellite(ctx)
	if err != nil {
		if ordersBySatellite == nil {
			service.log.Error("listing orders", zap.Error(err))
			return nil
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
	}

	close(requests)
	err = batchGroup.Wait()
	if err != nil {
		service.log.Error("archiving orders", zap.Error(err))
	}
	return nil
}

// Settle uploads orders to the satellite.
func (service *Service) Settle(ctx context.Context, satelliteID storj.NodeID, orders []*Info, requests chan ArchiveRequest) {
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

func (service *Service) settle(ctx context.Context, log *zap.Logger, satelliteID storj.NodeID, orders []*Info, requests chan ArchiveRequest) (err error) {
	defer mon.Task()(&ctx)(&err)

	log.Info("sending", zap.Int("count", len(orders)))
	defer log.Info("finished")

	address, err := service.trust.GetAddress(ctx, satelliteID)
	if err != nil {
		return OrderError.New("unable to get satellite address: %v", err)
	}
	satellite := pb.Node{
		Id: satelliteID,
		Address: &pb.NodeAddress{
			Transport: pb.NodeTransport_TCP_TLS_GRPC,
			Address:   address,
		},
	}

	conn, err := service.transport.DialNode(ctx, &satellite)
	if err != nil {
		return OrderError.New("unable to connect to the satellite: %v", err)
	}
	defer func() {
		if cerr := conn.Close(); cerr != nil {
			err = errs.Combine(err, OrderError.New("failed to close connection: %v", cerr))
		}
	}()

	client, err := pb.NewOrdersClient(conn).Settlement(ctx)
	if err != nil {
		return OrderError.New("failed to start settlement: %v", err)
	}

	var (
		errList errs.Group
		group   errgroup.Group
	)
	group.Go(func() error {
		for _, order := range orders {
			req := pb.SettlementRequest{
				Limit: order.Limit,
				Order: order.Order,
			}
			err := client.Send(&req)
			if err != nil {
				err = OrderError.New("sending settlement agreements returned an error: %v", err)
				log.Error("gRPC client when sending new orders settlements",
					zap.Error(err),
					zap.Any("request", req),
				)
				errList.Add(err)
				return nil
			}
		}

		err := client.CloseSend()
		if err != nil {
			err = OrderError.New("CloseSend settlement agreements returned an error: %v", err)
			log.Error("gRPC client error when closing sender ", zap.Error(err))
			errList.Add(err)
		}

		return nil
	})

	for {
		response, err := client.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}

			err = OrderError.New("failed to receive settlement response: %v", err)
			log.Error("gRPC client error when receiveing new order settlements", zap.Error(err))
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
			log.Error("gRPC client received a unexpected new orders setlement status",
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

	// Errors of this group are reported to errList so it always return nil
	_ = group.Wait()
	return errList.Err()
}

// Close stops the sending service.
func (service *Service) Close() error {
	service.Sender.Close()
	service.Cleanup.Close()
	return nil
}
