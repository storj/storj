// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore

import (
	"bytes"
	"context"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/errs2"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/signing"
)

var (
	// ErrVerifyNotAuthorized is returned when the one submitting the action is not authorized to perform that action.
	ErrVerifyNotAuthorized = errs.Class("not authorized")
	// ErrVerifyUntrusted is returned when action is not trusted.
	ErrVerifyUntrusted = errs.Class("untrusted")
	// ErrVerifyDuplicateRequest is returned when serial number has been already used to submit an action.
	ErrVerifyDuplicateRequest = errs.Class("duplicate request")
)

var monVerifyOrderLimit = mon.Task()

// VerifyOrderLimit verifies that the order limit is properly signed and has sane values.
// It also verifies that the serial number has not been used.
func (endpoint *Endpoint) verifyOrderLimit(ctx context.Context, limit *pb.OrderLimit) (err error) {
	defer monVerifyOrderLimit(&ctx)(&err)

	// sanity checks
	now := time.Now()
	switch {
	case limit.Limit < 0:
		return rpcstatus.Error(rpcstatus.InvalidArgument, "order limit is negative")
	case endpoint.ident.ID != limit.StorageNodeId:
		return rpcstatus.Errorf(rpcstatus.InvalidArgument, "order intended for other storagenode: %v", limit.StorageNodeId)
	case endpoint.IsExpired(limit.PieceExpiration):
		return rpcstatus.Errorf(rpcstatus.InvalidArgument, "piece expired: %v", limit.PieceExpiration)
	case endpoint.IsExpired(limit.OrderExpiration):
		return rpcstatus.Errorf(rpcstatus.InvalidArgument, "order expired: %v", limit.OrderExpiration)
	case now.Sub(limit.OrderCreation) > endpoint.config.OrderLimitGracePeriod:
		return rpcstatus.Errorf(rpcstatus.InvalidArgument, "order created too long ago: OrderCreation %v < SystemClock %v", limit.OrderCreation, now)
	case limit.OrderCreation.Sub(now) > endpoint.config.OrderLimitGracePeriod:
		return rpcstatus.Errorf(rpcstatus.InvalidArgument, "order created too far in the future: OrderCreation %v > SystemClock %v", limit.OrderCreation, now)
	case limit.SatelliteId.IsZero():
		return rpcstatus.Errorf(rpcstatus.InvalidArgument, "missing satellite id")
	case limit.UplinkPublicKey.IsZero():
		return rpcstatus.Errorf(rpcstatus.InvalidArgument, "missing uplink public key")
	case len(limit.SatelliteSignature) == 0:
		return rpcstatus.Errorf(rpcstatus.InvalidArgument, "missing satellite signature")
	case limit.PieceId.IsZero():
		return rpcstatus.Errorf(rpcstatus.InvalidArgument, "missing piece id")
	}

	if err := endpoint.trust.VerifySatelliteID(ctx, limit.SatelliteId); err != nil {
		return rpcstatus.Wrap(rpcstatus.PermissionDenied, err)
	}

	if err := endpoint.VerifyOrderLimitSignature(ctx, limit); err != nil {
		if errs2.IsCanceled(err) {
			return rpcstatus.Wrap(rpcstatus.Canceled, err)
		}
		return rpcstatus.Wrap(rpcstatus.Unauthenticated, err)
	}

	serialExpiration := limit.OrderExpiration

	// Expire the serial earlier if the grace period is smaller than the serial expiration.
	if graceExpiration := now.Add(endpoint.config.OrderLimitGracePeriod); graceExpiration.Before(serialExpiration) {
		serialExpiration = graceExpiration
	}

	if err := endpoint.usedSerials.Add(limit.SatelliteId, limit.SerialNumber, serialExpiration); err != nil {
		return rpcstatus.Wrap(rpcstatus.Unauthenticated, err)
	}

	return nil
}

var monVerifyOrder = mon.Task()

// VerifyOrder verifies that the order corresponds to the order limit and has all the necessary fields.
func (endpoint *Endpoint) VerifyOrder(ctx context.Context, limit *pb.OrderLimit, order *pb.Order, largestOrderAmount int64) (err error) {
	defer monVerifyOrder(&ctx)(&err)

	if order.SerialNumber != limit.SerialNumber {
		return rpcstatus.Error(rpcstatus.InvalidArgument, "order serial number changed during upload")
	}
	// TODO: add check for minimum allocation step
	if order.Amount < largestOrderAmount {
		return rpcstatus.Errorf(rpcstatus.InvalidArgument,
			"order contained smaller amount=%v, previous=%v",
			order.Amount, largestOrderAmount)
	}
	if order.Amount > limit.Limit {
		return rpcstatus.Errorf(rpcstatus.InvalidArgument,
			"order exceeded allowed amount=%v, limit=%v",
			order.Amount, limit.Limit)
	}

	if err := signing.VerifyUplinkOrderSignature(ctx, limit.UplinkPublicKey, order); err != nil {
		return rpcstatus.Wrap(rpcstatus.Unauthenticated, err)
	}

	return nil
}

// VerifyPieceHash verifies whether the piece hash is properly signed and matches the locally computed hash.
func (endpoint *Endpoint) VerifyPieceHash(ctx context.Context, limit *pb.OrderLimit, hash *pb.PieceHash, expectedHash []byte) (err error) {
	defer mon.Task()(&ctx)(&err)

	if limit == nil || hash == nil || len(expectedHash) == 0 {
		return rpcstatus.Error(rpcstatus.InvalidArgument, "invalid arguments")
	}
	if limit.PieceId != hash.PieceId {
		return rpcstatus.Error(rpcstatus.InvalidArgument, "piece id changed")
	}
	if !bytes.Equal(hash.Hash, expectedHash) {
		return rpcstatus.Error(rpcstatus.InvalidArgument, "hashes don't match")
	}

	if err := signing.VerifyUplinkPieceHashSignature(ctx, limit.UplinkPublicKey, hash); err != nil {
		return rpcstatus.Error(rpcstatus.Unauthenticated, "invalid piece hash signature")
	}

	return nil
}

var monVerifyOrderLimitSignature = mon.Task()

// VerifyOrderLimitSignature verifies that the order limit signature is valid.
func (endpoint *Endpoint) VerifyOrderLimitSignature(ctx context.Context, limit *pb.OrderLimit) (err error) {
	defer monVerifyOrderLimitSignature(&ctx)(&err)

	signee, err := endpoint.trust.GetSignee(ctx, limit.SatelliteId)
	if err != nil {
		if errs2.IsCanceled(err) {
			return rpcstatus.Wrap(rpcstatus.Canceled, err)
		}
		return rpcstatus.Wrap(rpcstatus.Unauthenticated,
			ErrVerifyUntrusted.New("unable to get signee: %w", err))
	}

	if err := signing.VerifyOrderLimitSignature(ctx, signee, limit); err != nil {
		return rpcstatus.Wrap(rpcstatus.Unauthenticated,
			ErrVerifyUntrusted.New("invalid order limit signature: %w", err))
	}

	return nil
}

// IsExpired checks whether the date has already expired (with a threshold) at the time of calling this function.
func (endpoint *Endpoint) IsExpired(expiration time.Time) bool {
	if expiration.IsZero() {
		return false
	}

	// TODO: return specific error about either exceeding the expiration completely or just the grace period
	return expiration.Before(time.Now().Add(-endpoint.config.ExpirationGracePeriod))
}
