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
	// Put inserts or updates a voucher from a satellite
	Put(context.Context, *pb.Voucher) error
	// GetExpiring retrieves all vouchers that are expired or about to expire
	GetExpiring(context.Context) ([]storj.NodeID, error)
	// GetValid returns one valid voucher from the list of approved satellites
	GetValid(context.Context, []storj.NodeID) (*pb.Voucher, error)
}
