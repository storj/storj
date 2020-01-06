// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package vouchers

import (
	"context"

	"github.com/zeebo/errs"

	"storj.io/common/pb"
)

// Endpoint for issuing signed vouchers (DEPRECATED)
//
// architecture: Endpoint
type Endpoint struct{}

// Request is deprecated and returns an error asking the storage node to update to the latest version.
func (endpoint *Endpoint) Request(ctx context.Context, req *pb.VoucherRequest) (_ *pb.VoucherResponse, err error) {
	return nil, errs.New("Vouchers endpoint is deprecated. Please upgrade your storage node to the latest version.")
}
