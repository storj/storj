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
)

var (
	// VoucherError represents errors with vouchers
	VoucherError = errs.Class("voucher")

	mon = monkit.Package()
)

// Service is a service for requesting 
type Service struct {
	log *zap.Logger

	kademlia *kademlia.Kademlia
	transport transport.Client

	// vdb vouchers.DB
	// archive orders.DB
	
	Loop sync2.Cycle
}

// NewService creates a new voucher service
func NewService(log *zap.Logger) *Service {
	return &Service{
		log: log,
		Loop: *sync2.NewCycle(24*7*time.Hour),
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

	// TODO: request first vouchers from new satellites

	return err
}

func (service *Service) renewVouchers(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	service.log.Debug("Getting vouchers close to expiration")

	expired, err := service.vdb.GetExpired(ctx)
	if err != nil {
		return err
	}

	if len(expired) > 0 {
		var group errgroup.Group
		ctx, cancel := context.WithTimeout(ctx, time.Hour)
		defer cancel()

		for satelliteID := range expired {
			err = service.request(ctx, satelliteID)
			if err != nil {
				return err
			}
		}
	} else {
		service.log.Debug("No vouchers close to expiration")
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