// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pointerdb

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/storj/pkg/certdb"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
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

	pba = &pb.PayerBandwidthAllocation{
		SatelliteId:       allocation.satelliteIdentity.ID,
		UplinkId:          peerIdentity.ID,
		CreatedUnixSec:    created,
		ExpirationUnixSec: created + int64(ttl),
		Action:            action,
		SerialNumber:      serialNum.String(),
	}
	return pba, nil
}
