// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package vouchers

import (
	"context"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/pb"
)

// Endpoint for issuing signed vouchers
type Endpoint struct{}

// Request receives a voucher request and returns a voucher and an error
func (endpoint *Endpoint) Request(ctx context.Context, req *pb.VoucherRequest) (_ *pb.VoucherResponse, err error) {
	return nil, errs.New("Vouchers endpoint is deprecated. Please upgrade your storage node to the latest version.")
}
