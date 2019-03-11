// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore

import (
	"bytes"
	"context"
	"time"

	"storj.io/storj/pkg/auth/signing"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
)

var (
	ErrVerifyNotAuthorized    = errs.Class("not authorized")
	ErrVerifyUntrusted        = errs.Class("untrusted")
	ErrVerifyDuplicateRequest = errs.Class("duplicate request")
)

func (endpoint *Endpoint) VerifyOrderLimit(ctx context.Context, limit *pb.OrderLimit2) error {
	// sanity checks
	switch {
	case limit.Limit < 0:
		return ErrProtocol.New("order limit is negative")
	case endpoint.signer.ID() != limit.StorageNodeId:
		return ErrProtocol.New("order intended for other storagenode: %v", limit.StorageNodeId)
	case endpoint.IsExpired(limit.PieceExpiration):
		return ErrProtocol.New("piece expired: %v", limit.PieceExpiration)
	case endpoint.IsExpired(limit.OrderExpiration):
		return ErrProtocol.New("order expired: %v", limit.OrderExpiration)

	case limit.SatelliteId.IsZero():
		return ErrProtocol.New("missing satellite id")
	case limit.UplinkId.IsZero():
		return ErrProtocol.New("missing uplink id")
	case len(limit.SatelliteSignature) == 0:
		return ErrProtocol.New("satellite signature missing")
	}

	// either uplink or satellite can only make the request
	// TODO: should this check be based on the action?
	//       with macaroons we might not have either of them doing the action
	peer, err := identity.PeerIdentityFromContext(ctx)
	if err != nil || limit.UplinkId != peer.ID && limit.SatelliteId != peer.ID {
		return ErrVerifyNotAuthorized.New("uplink:%s satellite:%s sender %s", limit.UplinkId, limit.SatelliteId, peer.ID)
	}

	if err := endpoint.trust.VerifySatelliteID(ctx, limit.SatelliteId); err != nil {
		return ErrVerifyUntrusted.Wrap(err)
	}
	if err := endpoint.trust.VerifyUplinkID(ctx, limit.UplinkId); err != nil {
		return ErrVerifyUntrusted.Wrap(err)
	}

	if err := endpoint.VerifyOrderLimitSignature(ctx, limit); err != nil {
		return ErrVerifyUntrusted.Wrap(err)
	}

	if ok := endpoint.activeSerials.Add(limit.SatelliteId, limit.SerialNumber, limit.OrderExpiration); !ok {
		return ErrVerifyDuplicateRequest.Wrap(err)
	}

	return nil
}

func (endpoint *Endpoint) VerifyOrder(ctx context.Context, peer *identity.PeerIdentity, limit *pb.OrderLimit2, order *pb.Order2, largestOrderAmount int64) error {
	// if order.SerialNumber != limit.SerialNumber {
	if !bytes.Equal(order.SerialNumber, limit.SerialNumber) {
		return ErrProtocol.New("order serial number changed during upload") // TODO: report grpc status bad message
	}
	if order.Amount < largestOrderAmount {
		return ErrProtocol.New("order contained smaller amount=%v, previous=%v", order.Amount, largestOrderAmount) // TODO: report grpc status bad message
	}
	if order.Amount > limit.Limit {
		return ErrProtocol.New("order exceeded allowed amount=%v, limit=%v", order.Amount, limit.Limit) // TODO: report grpc status bad message
	}

	if err := signing.VerifyOrderSignature(signing.SigneeFromPeerIdentity(peer), order); err != nil {
		return ErrVerifyUntrusted.New("invalid order signature") // TODO: report grpc status bad message
	}

	return nil
}

func (endpoint *Endpoint) VerifyPieceHash(ctx context.Context, peer *identity.PeerIdentity, limit *pb.OrderLimit2, hash *pb.PieceHash, expectedHash []byte) error {
	if peer == nil || limit == nil || hash == nil || len(expectedHash) == 0 {
		return ErrProtocol.New("invalid arguments")
	}
	if limit.PieceId != hash.PieceId {
		return ErrProtocol.New("piece id changed") // TODO: report grpc status bad message
	}
	if !bytes.Equal(hash.Hash, expectedHash) {
		return ErrProtocol.New("hashes don't match") // TODO: report grpc status bad message
	}

	if err := signing.VerifyPieceHashSignature(signing.SigneeFromPeerIdentity(peer), hash); err != nil {
		return ErrVerifyUntrusted.New("invalid hash signature: %v", err) // TODO: report grpc status bad message
	}

	return nil
}

func (endpoint *Endpoint) VerifyOrderLimitSignature(ctx context.Context, limit *pb.OrderLimit2) error {
	signee, err := endpoint.trust.GetSignee(ctx, limit.SatelliteId)
	if err != nil {
		return ErrVerifyUntrusted.New("unable to get signee: %v", err) // TODO: report grpc status bad message
	}

	if err := signing.VerifyOrderLimitSignature(signee, limit); err != nil {
		return ErrVerifyUntrusted.New("invalid order limit signature: %v", err) // TODO: report grpc status bad message
	}

	return nil
}

func (endpoint *Endpoint) IsExpired(expiration *timestamp.Timestamp) bool {
	if expiration == nil {
		return true
	}

	expirationTime, err := ptypes.Timestamp(expiration)
	if err != nil {
		// TODO: return error
		return true
	}

	// TODO: return specific error about either exceeding the expiration completely or just the grace period

	return expirationTime.After(time.Now().Add(-endpoint.config.ExpirationGracePeriod))
}
