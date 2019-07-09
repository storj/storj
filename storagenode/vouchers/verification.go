// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package vouchers

import (
	"context"
	"time"

	"github.com/zeebo/errs"

	"storj.io/storj/internal/errs2"
	"storj.io/storj/pkg/auth/signing"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

var (
	// ErrVerify is returned when voucher fields are not valid.
	ErrVerify = errs.Class("verification")
)

// VerifyVoucher verifies that the signature and the information contained in a voucher are valid
func (service *Service) VerifyVoucher(ctx context.Context, satellite storj.NodeID, voucher *pb.Voucher) (err error) {
	defer mon.Task()(&ctx)(&err)

	if self := service.kademlia.Local().Id; voucher.StorageNodeId != self {
		return ErrVerify.New("Storage node ID does not match expected: (%v) (%v)", voucher.StorageNodeId, self)
	}

	if voucher.SatelliteId != satellite {
		return ErrVerify.New("Satellite ID does not match expected: (%v) (%v)", voucher.SatelliteId, satellite)
	}

	if voucher.Expiration.Before(time.Now()) {
		return ErrVerify.New("Voucher is already expired")
	}

	signee, err := service.trust.GetSignee(ctx, voucher.SatelliteId)
	if err != nil {
		if errs2.IsCanceled(err) {
			return err
		}
		return ErrVerify.New("unable to get signee: %v", err) // TODO: report grpc status bad message
	}

	if err := signing.VerifyVoucher(ctx, signee, voucher); err != nil {
		return ErrVerify.New("invalid voucher signature: %v", err) // TODO: report grpc bad message
	}

	return nil
}
