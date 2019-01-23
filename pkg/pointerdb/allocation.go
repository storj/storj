// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pointerdb

import (
	"context"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/storj/pkg/auth"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
)

// AllocationSigner structure
type AllocationSigner struct {
	satelliteIdentity *identity.FullIdentity
	bwExpiration      int
}

// NewAllocationSigner creates new instance
func NewAllocationSigner(satelliteIdentity *identity.FullIdentity, bwExpiration int) *AllocationSigner {
	return &AllocationSigner{
		satelliteIdentity: satelliteIdentity,
		bwExpiration:      bwExpiration,
	}
}

// PayerBandwidthAllocation returns generated payer bandwidth allocation
func (allocation *AllocationSigner) PayerBandwidthAllocation(ctx context.Context, peerIdentity *identity.PeerIdentity, action pb.PayerBandwidthAllocation_Action) (pba *pb.PayerBandwidthAllocation, err error) {
	if peerIdentity == nil {
		return nil, Error.New("missing peer identity")
	}

	certs := [][]byte{allocation.satelliteIdentity.Leaf.Raw, allocation.satelliteIdentity.CA.Raw}
	certs = append(certs, allocation.satelliteIdentity.RestChainRaw()...) //todo:  do we need RestChain?

	serialNum, err := uuid.New()
	if err != nil {
		return nil, err
	}
	created := time.Now().Unix()

	// convert ttl from days to seconds
	ttl := allocation.bwExpiration
	ttl *= 86400

	pbad := &pb.PayerBandwidthAllocation_Data{
		SatelliteId:       allocation.satelliteIdentity.ID,
		UplinkId:          peerIdentity.ID,
		CreatedUnixSec:    created,
		ExpirationUnixSec: created + int64(ttl),
		Action:            action,
		SerialNumber:      serialNum.String(),
	}

	data, err := proto.Marshal(pbad)
	if err != nil {
		return nil, err
	}
	signature, err := auth.GenerateSignature(data, allocation.satelliteIdentity)
	if err != nil {
		return nil, err
	}
	return &pb.PayerBandwidthAllocation{Signature: signature, Data: data, Certs: certs}, nil
}
