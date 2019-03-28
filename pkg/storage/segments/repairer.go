// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package segments

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/auth/signing"
	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/storage/ec"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/orders"
)

// Repairer for segments
type Repairer struct {
	log *zap.Logger

	pointerdb  *pointerdb.Service
	allocation *pointerdb.AllocationSigner
	cache      *overlay.Cache
	orders     orders.DB
	ec         ecclient.Client
	signer     signing.Signer
	identity   *identity.FullIdentity
	timeout    time.Duration
}

// NewSegmentRepairer creates a new instance of SegmentRepairer
func NewSegmentRepairer(log *zap.Logger, pointerdb *pointerdb.Service, allocation *pointerdb.AllocationSigner, cache *overlay.Cache, ec ecclient.Client, identity *identity.FullIdentity, timeout time.Duration, orders orders.DB) *Repairer {
	return &Repairer{
		log: log,

		pointerdb:  pointerdb,
		allocation: allocation,
		cache:      cache,
		ec:         ec,
		identity:   identity,
		signer:     signing.SignerFromFullIdentity(identity),
		timeout:    timeout,
		orders:     orders,
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

	redundancy, err := eestream.NewRedundancyStrategyFromProto(pointer.GetRemote().GetRedundancy())
	if err != nil {
		return Error.Wrap(err)
	}

	pieceSize := eestream.CalcPieceSize(pointer.GetSegmentSize(), redundancy)
	rootPieceID := pointer.GetRemote().RootPieceId
	expiration := pointer.GetExpirationDate()

	var excludeNodeIDs storj.NodeIDList
	var healthyPieces []*pb.RemotePiece
	lostPiecesSet := sliceToSet(lostPieces)

	// Populate healthyPieces with all pieces from the pointer except those correlating to indices in lostPieces
	for _, piece := range pointer.GetRemote().GetRemotePieces() {
		excludeNodeIDs = append(excludeNodeIDs, piece.NodeId)
		if _, ok := lostPiecesSet[piece.GetPieceNum()]; !ok {
			healthyPieces = append(healthyPieces, piece)
		}
	}

	// Add Serial Number for the entire pointer audit
	// needs to be the same for all nodes in the pointer
	uuidGet, err := uuid.New()
	if err != nil {
		return err
	}
	serialNumberGet := storj.SerialNumber(*uuidGet)

	// Create the order limits for the GET_REPAIR action
	getLimits := make([]*pb.AddressedOrderLimit, redundancy.TotalCount())
	for _, piece := range healthyPieces {
		derivedPieceID := rootPieceID.Derive(piece.NodeId)
		orderLimit, err := repairer.createOrderLimit(ctx, serialNumberGet, piece.NodeId, derivedPieceID, expiration, pieceSize, pb.PieceAction_GET_REPAIR)
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
	request := overlay.FindStorageNodesRequest{
		RequestedCount: redundancy.TotalCount() - len(healthyPieces),
		FreeBandwidth:  pieceSize,
		FreeDisk:       pieceSize,
		ExcludedNodes:  excludeNodeIDs,
	}
	newNodes, err := repairer.cache.FindStorageNodes(ctx, request)
	if err != nil {
		return Error.Wrap(err)
	}

	// Add Serial Number for the entire pointer audit
	// needs to be the same for all nodes in the pointer
	uuidPut, err := uuid.New()
	if err != nil {
		return err
	}
	serialNumberPut := storj.SerialNumber(*uuidPut)

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
		orderLimit, err := repairer.createOrderLimit(ctx, serialNumberPut, node.Id, derivedPieceID, expiration, pieceSize, pb.PieceAction_PUT_REPAIR)
		if err != nil {
			return err
		}

		putLimits[pieceNum] = &pb.AddressedOrderLimit{
			Limit:              orderLimit,
			StorageNodeAddress: node.Address,
		}
		pieceNum++
	}

	// Note from Nat: I added the orders DB field to the repairer struct, but a log should be added, then passed here too
	endpoint := metainfo.NewEndpointOnSatellite(repairer.log, repairer.pointerdb, repairer.allocation, repairer.cache, signing.SignerFromFullIdentity(repairer.identity), repairer.orders)

	// projectID := `somehow find the project ID in an api-keyless way, aka make or find a different query if possible`
	// bucket := `find the bucket`

	// Note from Nat: I made SaveRemoteOrder exported so it could be used here
	// ToDo: Save Get Limits
	// err = endpoint.SaveRemoteOrder(ctx, projectID, bucket, getLimits)
	// if err != nil {
	// 	return nil, status.Errorf(codes.Internal, err.Error())
	// }

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

	//ToDo: Save Put Limits
	/*if err := endpoint.SaveRemoteOrder(ctx, keyInfo.ProjectID, req.Bucket, addressedLimits); err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}*/

	// Upload the repaired pieces
	successfulNodes, hashes, err := repairer.ec.Repair(ctx, putLimits, redundancy, r, convertTime(expiration), repairer.timeout)
	if err != nil {
		return Error.Wrap(err)
	}

	// Add the successfully uploaded pieces to the healthyPieces
	for i, node := range successfulNodes {
		if node == nil {
			continue
		}
		healthyPieces = append(healthyPieces, &pb.RemotePiece{
			PieceNum: int32(i),
			NodeId:   node.Id,
			Hash:     hashes[i],
		})
	}

	// Update the remote pieces in the pointer
	pointer.GetRemote().RemotePieces = healthyPieces

	// Update the segment pointer in the PointerDB
	return repairer.pointerdb.Put(path, pointer)
}

func (repairer *Repairer) createOrderLimit(ctx context.Context, serialnumber storj.SerialNumber, nodeID storj.NodeID, pieceID storj.PieceID, expiration *timestamp.Timestamp, limit int64, action pb.PieceAction) (*pb.OrderLimit2, error) {
	parameters := pointerdb.OrderLimitParameters{
		SerialNumber:    serialnumber,
		UplinkIdentity:  repairer.identity.PeerIdentity(),
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

// sliceToSet converts the given slice to a set
func sliceToSet(slice []int32) map[int32]struct{} {
	set := make(map[int32]struct{}, len(slice))
	for _, value := range slice {
		set[value] = struct{}{}
	}
	return set
}
