// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package vouchers

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/auth/signing"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
)

// Config contains voucher endpoint configuration parameters
type Config struct {
	Expiration time.Duration `help:"length of time before a voucher expires" default:"720h0m0s"`
}

// Endpoint for issuing signed vouchers
type Endpoint struct {
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

// NewEndpoint creates a new endpoint for issuing signed vouchers
func NewEndpoint(log *zap.Logger, satellite signing.Signer, cache *overlay.Cache, expiration time.Duration) *Endpoint {
	return &Endpoint{
		log:        log,
		satellite:  satellite,
		cache:      cache,
		expiration: expiration,
	}
}

// Request receives a voucher request and returns a voucher and an error
func (endpoint *Endpoint) Request(ctx context.Context, req *pb.VoucherRequest) (_ *pb.VoucherResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	peer, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	reputable, err := endpoint.cache.IsVetted(ctx, peer.ID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	endpoint.log.Debug("Node reputation", zap.Bool("reputable", reputable))

	if !reputable {
		return &pb.VoucherResponse{Status: pb.VoucherResponse_REJECTED}, nil
	}

	unsigned := &pb.Voucher{
		SatelliteId:   endpoint.satellite.ID(),
		StorageNodeId: peer.ID,
		Expiration:    time.Now().Add(endpoint.expiration),
	}

	voucher, err := signing.SignVoucher(ctx, endpoint.satellite, unsigned)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return &pb.VoucherResponse{
		Voucher: voucher,
		Status:  pb.VoucherResponse_ACCEPTED,
	}, nil
}
