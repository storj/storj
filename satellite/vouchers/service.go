// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package vouchers

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/auth/signing"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
)

// Config contains voucher service configuration parameters
type Config struct {
	Expiration int `help:"number of days before a voucher expires" default:"30"`
}

// Service for issuing signed vouchers
type Service struct {
	log        *zap.Logger
	satellite  signing.Signer
	cache      *overlay.Cache
	expiration time.Duration
}

var (
	// Error the default vouchers errs class
	Error = errs.Class("vouchers error")

	mon = monkit.Package()
)

// NewService creates a new service for issuing signed vouchers
func NewService(log *zap.Logger, satellite signing.Signer, cache *overlay.Cache, expiration time.Duration) *Service {
	return &Service{
		log:        log,
		satellite:  satellite,
		cache:      cache,
		expiration: expiration,
	}
}

// Request receives a voucher request and returns a voucher and an error
func (service *Service) Request(ctx context.Context, req *pb.VoucherRequest) (_ *pb.VoucherResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	peer, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	reputable, err := service.cache.IsVetted(ctx, peer.ID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	service.log.Debug("Node reputation", zap.Bool("reputable", reputable))

	if !reputable {
		return &pb.VoucherResponse{Status: pb.VoucherResponse_REJECTED}, nil
	}

	expirationTime := time.Now().UTC().Add(service.expiration)
	expiration, err := ptypes.TimestampProto(expirationTime)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	unsigned := &pb.Voucher{
		SatelliteId:   service.satellite.ID(),
		StorageNodeId: peer.ID,
		Expiration:    expiration,
	}

	voucher, err := signing.SignVoucher(ctx, service.satellite, unsigned)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return &pb.VoucherResponse{
		Voucher: voucher,
		Status:  pb.VoucherResponse_ACCEPTED,
	}, nil
}
