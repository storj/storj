// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package vouchers

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/auth/signing"
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
func (service *Service) Request(ctx context.Context, req *pb.VoucherRequest) (*pb.Voucher, error) {
	return &pb.Voucher{}, errs.New("Voucher service not implemented")
}
