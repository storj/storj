// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pointerdb

import (
	"context"
	"errors"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/protobuf/ptypes"

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
func (allocation *AllocationSigner) PayerBandwidthAllocation(ctx context.Context, peerIdentity *identity.PeerIdentity, action pb.BandwidthAction) (pba *pb.OrderLimit, err error) {
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

	pba = &pb.OrderLimit{
		SatelliteId:       allocation.satelliteIdentity.ID,
		UplinkId:          peerIdentity.ID,
		CreatedUnixSec:    created,
		ExpirationUnixSec: created + int64(ttl),
		Action:            action,
		SerialNumber:      serialNum.String(),
	}
	err = auth.SignMessage(pba, *allocation.satelliteIdentity)
	return pba, err
}

// OrderLimit returns generated order limit
func (allocation *AllocationSigner) OrderLimit(ctx context.Context, uplinkIdentity *identity.PeerIdentity, storageNodeID storj.NodeID, pieceID storj.PieceID2, action pb.Action) (pba *pb.OrderLimit2, err error) {
	if uplinkIdentity == nil {
		return nil, Error.New("missing uplink identity")
	}
	serialNum, err := uuid.New()
	if err != nil {
		return nil, err
	}

	// store the corresponding uplink's id and public key into certDB db
	err = allocation.certdb.SavePublicKey(ctx, uplinkIdentity.ID, uplinkIdentity.Leaf.PublicKey)
	if err != nil {
		return nil, err
	}

	if err := allocation.restrictActionsOrderLimit(uplinkIdentity.ID, action); err != nil {
		return nil, err
	}

	// convert bwExpiration from days to seconds
	orderExpiration, err := ptypes.TimestampProto(time.Unix(int64(allocation.bwExpiration*86400), 0))
	if err != nil {
		return nil, err
	}
	pba = &pb.OrderLimit2{
		SerialNumber:  []byte(serialNum.String()),
		SatelliteId:   allocation.satelliteIdentity.ID,
		UplinkId:      uplinkIdentity.ID,
		StorageNodeId: storageNodeID,
		PieceId:       pieceID,
		Action:        action,
		// Limit: 0,
		// PieceExpiration:
		OrderExpiration: orderExpiration,
	}

	msgBytes, err := proto.Marshal(pba)
	if err != nil {
		return nil, auth.ErrMarshal.Wrap(err)
	}
	signeture, err := auth.GenerateSignature(msgBytes, allocation.satelliteIdentity)
	if err != nil {
		return nil, auth.ErrMarshal.Wrap(err)
	}
	pba.SatelliteSignature = signeture

	return pba, err
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

func (allocation *AllocationSigner) restrictActionsOrderLimit(peerID storj.NodeID, action pb.Action) error {
	switch action {
	case pb.Action_GET_REPAIR, pb.Action_PUT_REPAIR, pb.Action_GET_AUDIT:
		if peerID != allocation.satelliteIdentity.ID {
			return errors.New("action restricted to signing satellite")
		}

		return nil
	case pb.Action_GET, pb.Action_PUT, pb.Action_DELETE:
		return nil
	default:
		return errors.New("unknown action restriction")
	}
}
