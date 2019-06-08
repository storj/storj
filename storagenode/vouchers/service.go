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
	// GetExpiring retrieves all vouchers that are expired or about to expire
	GetExpiring(context.Context, time.Duration) ([]storj.NodeID, error)
	// GetValid returns one valid voucher from the list of approved satellites
	GetValid(context.Context, []storj.NodeID) (*pb.Voucher, error)
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
func NewService(log *zap.Logger, interval, expirationBuffer time.Duration) *Service {

	return &Service{
		log:              log,
		expirationBuffer: expirationBuffer,
		Loop:             *sync2.NewCycle(interval),
	}
}

// Run sends requests to satellites for vouchers
func (service *Service) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	return service.Loop.Run(ctx, service.runOnce)
}

func (service *Service) runOnce(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	// request vouchers for entries that are expired/about to expire
	err = service.renewVouchers(ctx)

	// request first vouchers from new satellites which have no voucher
	err = service.initialVouchers(ctx)
	return err
}

func (service *Service) renewVouchers(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	service.log.Debug("Getting vouchers close to expiration")

	expired, err := service.vdb.GetExpiring(ctx, service.expirationBuffer)
	if err != nil {
		return err
	}

	if len(expired) > 0 {
		var group errgroup.Group
		ctx, cancel := context.WithTimeout(ctx, time.Hour)
		defer cancel()

		for _, satelliteID := range expired {
			satelliteID := satelliteID
			group.Go(func() error {
				err = service.request(ctx, satelliteID)
				if err != nil {
					service.log.Error("Error requesting voucher", zap.String("satellite", satelliteID.String()), zap.Error(err))
				}
				return nil
			})
		}
	} else {
		service.log.Debug("No vouchers close to expiration")
	}
	return err
}

func (service *Service) initialVouchers(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	withoutVouchers, err := service.getWithoutVouchers(ctx)
	if err != nil {
		return err
	}

	if len(withoutVouchers) > 0 {
		var group errgroup.Group
		ctx, cancel := context.WithTimeout(ctx, time.Hour)
		defer cancel()

		for _, satelliteID := range withoutVouchers {
			satelliteID := satelliteID
			group.Go(func() error {
				err = service.request(ctx, satelliteID)
				if err != nil {
					service.log.Error("Error requesting voucher", zap.String("satellite", satelliteID.String()), zap.Error(err))
				}
				return nil
			})
		}
	} else {
		service.log.Debug("No satellites requiring initial vouchers")
	}
	return err
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

	voucher, err := pb.NewVouchersClient(conn).Request(ctx, &pb.VoucherRequest{})
	if err != nil {
		return VoucherError.New("failed to start request: %v", err)
	}

	// check voucher fields

	return service.vdb.Put(ctx, voucher)
}

func (service *Service) getWithoutVouchers(ctx context.Context) (withoutVouchers []storj.NodeID, err error) {
	defer mon.Task()(&ctx)(&err)

	// get all satellite IDs from archive
	allSatellites, err := service.archive.ListSatellites(ctx)
	if err != nil {
		return nil, err
	}
	if len(allSatellites) == 0 {
		return withoutVouchers, nil
	}

	// get all satellites with vouchers
	withVouchers, err := service.vdb.ListSatellites(ctx)
	if err != nil {
		return nil, err
	}

	// insert all satellites with vouchers into a map for easy filtering
	voucherMap := make(map[storj.NodeID]bool)
	for _, sat := range withVouchers {
		voucherMap[sat] = true
	}

	// filter out satellites with vouchers
	for _, sat := range allSatellites {
		if voucherMap[sat] == false {
			withoutVouchers = append(withoutVouchers, sat)
		}
	}
	return withoutVouchers, nil
}
