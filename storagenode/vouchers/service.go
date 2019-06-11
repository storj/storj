// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package vouchers

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/sync2"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/storagenode/orders"
)

var (
	// VoucherError represents errors with vouchers
	VoucherError = errs.Class("voucher")

	mon = monkit.Package()
)

// DB implements storing and retrieving vouchers
type DB interface {
	// Put inserts or updates a voucher from a satellite
	Put(context.Context, *pb.Voucher) error
	// GetValid returns one valid voucher from the list of approved satellites
	GetValid(context.Context, []storj.NodeID) (*pb.Voucher, error)
	// NeedVoucher
	NeedVoucher(context.Context, storj.NodeID, time.Duration) (bool, error)
	// ListSatellites returns all satellites from the vouchersDB
	ListSatellites(context.Context) ([]storj.NodeID, error)
}

// Config defines configuration for requesting vouchers.
type Config struct {
	Interval         int `help:"number of days between voucher service iterations" default:"7"`
	ExpirationBuffer int `help:"buffer period of X days into the future. If a voucher would expire within this period, send a request for renewal" default:"7"`
}

// Service is a service for requesting vouchers
type Service struct {
	log *zap.Logger

	kademlia  *kademlia.Kademlia
	transport transport.Client

	vdb     DB
	archive orders.DB

	expirationBuffer time.Duration

	Loop sync2.Cycle
}

// NewService creates a new voucher service
func NewService(log *zap.Logger, kad *kademlia.Kademlia, transport transport.Client, vdb DB, archive orders.DB, interval, expirationBuffer time.Duration) *Service {
	return &Service{
		log:              log,
		kademlia:         kad,
		transport:        transport,
		vdb:              vdb,
		archive:          archive,
		expirationBuffer: expirationBuffer,
		Loop:             *sync2.NewCycle(interval),
	}
}

// Run sends requests to satellites for vouchers
func (service *Service) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	return service.Loop.Run(ctx, service.RunOnce)
}

// RunOnce runs one iteration of the voucher request service
func (service *Service) RunOnce(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	service.log.Info("Checking vouchers")

	allSatellites, err := service.archive.ListSatellites(ctx)
	if err != nil {
		service.log.Error("listing satellites", zap.Error(err))
		return nil
	}

	if len(allSatellites) > 0 {
		var group errgroup.Group
		ctx, cancel := context.WithTimeout(ctx, time.Hour)
		defer cancel()
		for _, sat := range allSatellites {
			sat := sat
			need, err := service.vdb.NeedVoucher(ctx, sat, service.expirationBuffer)
			if err != nil {
				service.log.Error("getting voucher status", zap.Error(err))
				return nil
			}
			if need {
				group.Go(func() error {
					service.Request(ctx, sat)
					return nil
				})
			}
		}
		_ = group.Wait() // doesn't return errors
	} else {
		service.log.Debug("No voucher requests to send")
	}
	return nil
}

// Request makes a voucher request to a satellite
func (service *Service) Request(ctx context.Context, satelliteID storj.NodeID) {
	service.log.Info("Requesting voucher", zap.String("satellite", satelliteID.String()))
	err := service.request(ctx, satelliteID)
	if err != nil {
		service.log.Error("Error requesting voucher", zap.String("satellite", satelliteID.String()), zap.Error(err))
	}
}

func (service *Service) request(ctx context.Context, satelliteID storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)

	satellite, err := service.kademlia.FindNode(ctx, satelliteID)
	if err != nil {
		return VoucherError.New("unable to find satellite on the network: %v", err)
	}

	conn, err := service.transport.DialNode(ctx, &satellite)
	if err != nil {
		return VoucherError.New("unable to connect to the satellite: %v", err)
	}
	defer func() {
		if cerr := conn.Close(); cerr != nil {
			err = errs.Combine(err, VoucherError.New("failed to close connection: %v", err))
		}
	}()

	resp, err := pb.NewVouchersClient(conn).Request(ctx, &pb.VoucherRequest{})
	if err != nil {
		return VoucherError.New("failed to start request: %v", err)
	}

	// TODO: separate status for disqualified nodes?
	switch resp.GetStatus() {
	case pb.VoucherResponse_REJECTED:
		service.log.Info("Voucher request denied. Node does not meet voucher requirements")
	case pb.VoucherResponse_ACCEPTED:
		voucher := resp.GetVoucher()
		// TODO: check voucher fields
		err = service.vdb.Put(ctx, voucher)
		if err != nil {
			return err
		}
		service.log.Info("Voucher received", zap.String("satellite", voucher.SatelliteId.String()))
	default:
		service.log.Warn("Unknown voucher response status")
	}

	return service.vdb.Put(ctx, voucher)
}

// Close stops the voucher service
func (service *Service) Close() error {
	service.Loop.Close()
	return nil
}
