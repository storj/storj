// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pointerdb

import (
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/storj/pkg/auth"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/peertls"
)

// Allocation structure
type Allocation struct {
	satelliteIdentity *identity.FullIdentity
	bwExpiration      int
}

// NewAllocation creates new instance
func NewAllocation(satelliteIdentity *identity.FullIdentity, bwExpiration int) *Allocation {
	return &Allocation{
		satelliteIdentity: satelliteIdentity,
		bwExpiration:      bwExpiration,
	}
}

// PayerBandwidthAllocation returns generated payer bandwidth allocation
func (allocation *Allocation) PayerBandwidthAllocation(ctx context.Context, peerIdentity *identity.PeerIdentity, action pb.PayerBandwidthAllocation_Action) (pba *pb.PayerBandwidthAllocation, err error) {
	if peerIdentity == nil {
		return nil, Error.New("missing peer identity")
	}

	pk, ok := peerIdentity.Leaf.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, peertls.ErrUnsupportedKey.New("%T", peerIdentity.Leaf.PublicKey)
	}

	pubbytes, err := x509.MarshalPKIXPublicKey(pk)
	if err != nil {
		return nil, err
	}

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
		PubKey:            pubbytes,
	}

	data, err := proto.Marshal(pbad)
	if err != nil {
		return nil, err
	}
	signature, err := auth.GenerateSignature(data, allocation.satelliteIdentity)
	if err != nil {
		return nil, err
	}
	return &pb.PayerBandwidthAllocation{Signature: signature, Data: data}, nil
}
