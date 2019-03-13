// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package segments

import (
	"context"

	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/auth/signing"
	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb"
	ecclient "storj.io/storj/pkg/storage/ec"
	"storj.io/storj/pkg/storj"
)

// Repairer for segments
type Repairer struct {
	pointerdb            *pointerdb.Service
	allocation           *pointerdb.AllocationSigner
	cache                *overlay.Cache
	ec                   ecclient.Client
	selectionPreferences *overlay.NodeSelectionConfig
	signer               signing.Signer
}

// NewSegmentRepairer creates a new instance of SegmentRepairer
func NewSegmentRepairer(pointerdb *pointerdb.Service, allocation *pointerdb.AllocationSigner, cache *overlay.Cache, ec ecclient.Client, signer signing.Signer, selectionPreferences *overlay.NodeSelectionConfig) *Repairer {
	return &Repairer{
		pointerdb:            pointerdb,
		allocation:           allocation,
		cache:                cache,
		ec:                   ec,
		signer:               signer,
		selectionPreferences: selectionPreferences,
	}
}

// Repair retrieves an at-risk segment and repairs and stores lost pieces on new nodes
func (repairer *Repairer) Repair(ctx context.Context, path storj.Path, lostPieces []int32) (err error) {
	defer mon.Task()(&ctx)(&err)

	// Read the segment pointer from the PointerDB
	pointer, err := repairer.pointerdb.Get(path)
	if err != nil {
		return Error.Wrap(err)
	}

	if pointer.GetType() != pb.Pointer_REMOTE {
		return Error.New("cannot repair inline segment %s", path)
	}

	repairerIdentity, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		return Error.Wrap(err)
	}

	redundancy, err := eestream.NewRedundancyStrategyFromProto(pointer.GetRemote().GetRedundancy())
	if err != nil {
		return Error.Wrap(err)
	}

	pieceSize := eestream.CalcPieceSize(pointer.GetSegmentSize(), redundancy)
	rootPieceID := pointer.GetRemote().PieceId_2
	expiration := pointer.GetExpirationDate()

	var excludeNodeIDs storj.NodeIDList
	var healthyPieces []*pb.RemotePiece

	// Populate healthyPieces with all pieces from the pointer except those correlating to indices in lostPieces
	for _, piece := range pointer.GetRemote().GetRemotePieces() {
		excludeNodeIDs = append(excludeNodeIDs, piece.NodeId)
		if !contains(lostPieces, piece.GetPieceNum()) {
			healthyPieces = append(healthyPieces, piece)
		}
	}

	// Create the order limits for the GET_REPAIR action
	getLimits := make([]*pb.AddressedOrderLimit, redundancy.TotalCount())
	for _, piece := range healthyPieces {
		derivedPieceID := rootPieceID.Derive(piece.NodeId)
		orderLimit, err := repairer.createOrderLimit(ctx, repairerIdentity, piece.NodeId, derivedPieceID, expiration, pieceSize, pb.Action_GET_REPAIR)
		if err != nil {
			return err
		}

		node, err := repairer.cache.Get(ctx, piece.NodeId)
		if err != nil {
			return Error.Wrap(err)
		}

		if node != nil {
			node.Type.DPanicOnInvalid("repair")
		}

		getLimits[piece.GetPieceNum()] = &pb.AddressedOrderLimit{
			Limit:              orderLimit,
			StorageNodeAddress: node.Address,
		}
	}

	// Request Overlay for n-h new storage nodes
	request := &pb.FindStorageNodesRequest{
		Opts: &pb.OverlayOptions{
			Amount: int64(redundancy.TotalCount()) - int64(len(healthyPieces)),
			Restrictions: &pb.NodeRestrictions{
				FreeBandwidth: pieceSize,
				FreeDisk:      pieceSize,
			},
			ExcludedNodes: excludeNodeIDs,
		},
	}
	newNodes, err := repairer.cache.FindStorageNodes(ctx, request, repairer.selectionPreferences)
	if err != nil {
		return Error.Wrap(err)
	}

	// Create the order limits for the PUT_REPAIR action
	putLimits := make([]*pb.AddressedOrderLimit, redundancy.TotalCount())
	pieceNum := 0
	for _, node := range newNodes {
		if node != nil {
			node.Type.DPanicOnInvalid("repair 2")
		}

		for pieceNum < redundancy.TotalCount() && getLimits[pieceNum] != nil {
			pieceNum++
		}

		if pieceNum >= redundancy.TotalCount() {
			break // should not happen
		}

		derivedPieceID := rootPieceID.Derive(node.Id)
		orderLimit, err := repairer.createOrderLimit(ctx, repairerIdentity, node.Id, derivedPieceID, expiration, pieceSize, pb.Action_GET_REPAIR)
		if err != nil {
			return err
		}

		putLimits[pieceNum] = &pb.AddressedOrderLimit{
			Limit:              orderLimit,
			StorageNodeAddress: node.Address,
		}
	}

	// Download the segment using just the healthy pieces
	rr, err := repairer.ec.Get(ctx, getLimits, redundancy, pointer.GetSegmentSize())
	if err != nil {
		return Error.Wrap(err)
	}

	r, err := rr.Range(ctx, 0, rr.Size())
	if err != nil {
		return Error.Wrap(err)
	}
	defer func() { err = errs.Combine(err, r.Close()) }()

	// Upload the repaired pieces
	successfulNodes, hashes, err := repairer.ec.Put(ctx, putLimits, redundancy, r, convertTime(expiration))
	if err != nil {
		return Error.Wrap(err)
	}

	// Add the successfully uploaded pieces to the healthyPieces
	for i, node := range successfulNodes {
		if node == nil {
			// copy the successfuNode info
			healthyPieces = append(healthyPieces, &pb.RemotePiece{
				PieceNum: int32(i),
				NodeId:   node.Id,
				Hash:     hashes[i],
			})
		}
	}

	// Update the remote pieces in the pointer
	pointer.GetRemote().RemotePieces = healthyPieces

	// Update the segment pointer in the PointerDB
	return repairer.pointerdb.Put(path, pointer)
}

func (repairer *Repairer) createOrderLimit(ctx context.Context, repairerIdentity *identity.PeerIdentity, nodeID storj.NodeID, pieceID pb.PieceID, expiration *timestamp.Timestamp, limit int64, action pb.Action) (*pb.OrderLimit2, error) {
	parameters := pointerdb.OrderLimitParameters{
		UplinkIdentity:  repairerIdentity,
		StorageNodeID:   nodeID,
		PieceID:         pieceID,
		Action:          action,
		PieceExpiration: expiration,
		Limit:           limit,
	}

	orderLimit, err := repairer.allocation.OrderLimit(ctx, parameters)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	orderLimit, err = signing.SignOrderLimit(repairer.signer, orderLimit)
	return orderLimit, Error.Wrap(err)
}

// contains checks if n exists in list
func contains(list []int32, n int32) bool {
	for i := range list {
		if n == list[i] {
			return true
		}
	}
	return false
}
