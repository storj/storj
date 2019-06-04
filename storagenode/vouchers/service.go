// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package vouchers

import (
	"context"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

// DB implements storing and retrieving vouchers
type DB interface {
	Put(context.Context, *pb.Voucher) error
	GetExpiring(context.Context) ([]storj.NodeID, error)
	PresentVoucher(context.Context, []storj.NodeID) (*pb.Voucher, error)
}
