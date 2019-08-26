// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package vouchers

import (
	"context"
	"time"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

// DB implements storing and retrieving vouchers
type DB interface {
	// Put inserts or updates a voucher from a satellite
	Put(context.Context, *pb.Voucher) error
	// GetAll returns all vouchers in the table
	GetAll(context.Context) ([]*pb.Voucher, error)
	// NeedVoucher returns true if a voucher from a particular satellite is expired, about to expire, or does not exist
	NeedVoucher(context.Context, storj.NodeID, time.Duration) (bool, error)
}

