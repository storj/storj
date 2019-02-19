// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pointerdb

import (
	"context"
	"errors"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/storj/pkg/auth"
	"storj.io/storj/pkg/certdb"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

// AllocationSigner structure
type AllocationSigner struct {
	satelliteIdentity *identity.FullIdentity
	bwExpiration      int
	certdb            certdb.DB
}

// NewAllocationSigner creates new instance
func NewAllocationSigner(satelliteIdentity *identity.FullIdentity, bwExpiration int, upldb certdb.DB) *AllocationSigner {
	return &AllocationSigner{
		satelliteIdentity: satelliteIdentity,
		bwExpiration:      bwExpiration,
		certdb:            upldb,
	}
}

// PayerBandwidthAllocation returns generated payer bandwidth allocation
func (allocation *AllocationSigner) PayerBandwidthAllocation(ctx context.Context, peerIdentity *identity.PeerIdentity, action pb.BandwidthAction) (pba *pb.PayerBandwidthAllocation, err error) {
	if peerIdentity == nil {
		return nil, Error.New("missing peer identity")
	}
	serialNum, err := uuid.New()
	if err != nil {
		return nil, err
	}
	created := time.Now().Unix()
	// convert ttl from days to seconds
	ttl := allocation.bwExpiration
	ttl *= 86400

	// store the corresponding uplink's id and public key into certDB db
	err = allocation.certdb.SavePublicKey(ctx, peerIdentity.ID, peerIdentity.Leaf.PublicKey)
	if err != nil {
		return nil, err
	}

	if err := allocation.restrictActions(peerIdentity.ID, action); err != nil {
		return nil, err
	}

	pba = &pb.PayerBandwidthAllocation{
		SatelliteId:       allocation.satelliteIdentity.ID,
		UplinkId:          peerIdentity.ID,
		CreatedUnixSec:    created,
		ExpirationUnixSec: created + int64(ttl),
		Action:            action,
		SerialNumber:      serialNum.String(),
	}
	if err := auth.SignMessage(pba, *allocation.satelliteIdentity); err != nil {
		return nil, err
	}
	return pba, nil
}

func (allocation *AllocationSigner) restrictActions(peerID storj.NodeID, action pb.BandwidthAction) error {
	switch action {
	case pb.BandwidthAction_GET_REPAIR, pb.BandwidthAction_PUT_REPAIR, pb.BandwidthAction_GET_AUDIT:
		if peerID != allocation.satelliteIdentity.ID {
			return errors.New("action restricted to signing satellite")
		}

		return nil
	case pb.BandwidthAction_GET, pb.BandwidthAction_PUT:
		return nil
	default:
		return errors.New("unknown action restriction")
	}
}
