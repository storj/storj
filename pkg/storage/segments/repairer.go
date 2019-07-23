// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package segments

import (
	"context"
	"math"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	ecclient "storj.io/storj/pkg/storage/ec"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/orders"
)

// Repairer for segments
type Repairer struct {
	log      *zap.Logger
	metainfo *metainfo.Service
	orders   *orders.Service
	cache    *overlay.Cache
	ec       ecclient.Client
	identity *identity.FullIdentity
	timeout  time.Duration

	// multiplierOptimalThreshold is the value that multiplied by the optimal
	// threshold results in the maximum limit of number of nodes to upload
	// repaired pieces
	multiplierOptimalThreshold float64
}

// NewSegmentRepairer creates a new instance of SegmentRepairer.
//
// excessPercentageOptimalThreshold is the percentage to apply over the optimal
// threshould to determine the maximum limit of nodes to upload repaired pieces,
// when negative, 0 is applied.
func NewSegmentRepairer(
	log *zap.Logger, metainfo *metainfo.Service, orders *orders.Service,
	cache *overlay.Cache, ec ecclient.Client, identity *identity.FullIdentity, timeout time.Duration,
	excessOptimalThreshold float64,
) *Repairer {

	if excessOptimalThreshold < 0 {
		excessOptimalThreshold = 0
	}

	return &Repairer{
		log:                        log,
		metainfo:                   metainfo,
		orders:                     orders,
		cache:                      cache,
		ec:                         ec.WithForceErrorDetection(true),
		identity:                   identity,
		timeout:                    timeout,
		multiplierOptimalThreshold: 1 + excessOptimalThreshold,
	}
}

// Repair retrieves an at-risk segment and repairs and stores lost pieces on new nodes
func (repairer *Repairer) Repair(ctx context.Context, path storj.Path) (err error) {
	defer mon.Task()(&ctx, path)(&err)

	// Read the segment pointer from the metainfo
	pointer, err := repairer.metainfo.Get(ctx, path)
	if err != nil {
		return Error.Wrap(err)
	}

	if pointer.GetType() != pb.Pointer_REMOTE {
		return Error.New("cannot repair inline segment %s", path)
	}

	mon.Meter("repair_attempts").Mark(1)
	mon.IntVal("repair_segment_size").Observe(pointer.GetSegmentSize())

	redundancy, err := eestream.NewRedundancyStrategyFromProto(pointer.GetRemote().GetRedundancy())
	if err != nil {
		return Error.Wrap(err)
	}

	pieceSize := eestream.CalcPieceSize(pointer.GetSegmentSize(), redundancy)
	expiration := pointer.GetExpirationDate()

	var excludeNodeIDs storj.NodeIDList
	var healthyPieces, unhealthyPieces []*pb.RemotePiece
	healthyMap := make(map[int32]bool)
	pieces := pointer.GetRemote().GetRemotePieces()
	missingPieces, err := repairer.cache.GetMissingPieces(ctx, pieces)
	if err != nil {
		return Error.New("error getting missing pieces %s", err)
	}

	numHealthy := len(pieces) - len(missingPieces)
	// irreparable piece, we need k+1 to detect corrupted pieces
	if int32(numHealthy) < pointer.Remote.Redundancy.MinReq+1 {
		mon.Meter("repair_nodes_unavailable").Mark(1)
		return Error.New("segment %v cannot be repaired: only %d healthy pieces, %d required", path, numHealthy, pointer.Remote.Redundancy.MinReq+1)
	}

	// repair not needed
	if int32(numHealthy) > pointer.Remote.Redundancy.RepairThreshold {
		mon.Meter("repair_unnecessary").Mark(1)
		repairer.log.Sugar().Debugf("segment %v with %d pieces above repair threshold %d", path, numHealthy, pointer.Remote.Redundancy.RepairThreshold)
		return nil
	}

	healthyRatioBeforeRepair := 0.0
	if pointer.Remote.Redundancy.Total != 0 {
		healthyRatioBeforeRepair = float64(numHealthy) / float64(pointer.Remote.Redundancy.Total)
	}
	mon.FloatVal("healthy_ratio_before_repair").Observe(healthyRatioBeforeRepair)

	lostPiecesSet := sliceToSet(missingPieces)

	// Populate healthyPieces with all pieces from the pointer except those correlating to indices in lostPieces
	for _, piece := range pieces {
		excludeNodeIDs = append(excludeNodeIDs, piece.NodeId)
		if _, ok := lostPiecesSet[piece.GetPieceNum()]; !ok {
			healthyPieces = append(healthyPieces, piece)
			healthyMap[piece.GetPieceNum()] = true
		} else {
			unhealthyPieces = append(unhealthyPieces, piece)
		}
	}

	bucketID, err := createBucketID(path)
	if err != nil {
		return Error.Wrap(err)
	}

	// Create the order limits for the GET_REPAIR action
	getOrderLimits, getPrivateKey, err := repairer.orders.CreateGetRepairOrderLimits(ctx, bucketID, pointer, healthyPieces)
	if err != nil {
		return Error.Wrap(err)
	}

	var requestCount int
	{
		totalNeeded := math.Ceil(float64(redundancy.OptimalThreshold()) *
			repairer.multiplierOptimalThreshold,
		)
		requestCount = int(totalNeeded) - len(healthyPieces)
	}

	// Request Overlay for n-h new storage nodes
	request := overlay.FindStorageNodesRequest{
		RequestedCount: requestCount,
		FreeBandwidth:  pieceSize,
		FreeDisk:       pieceSize,
		ExcludedNodes:  excludeNodeIDs,
	}
	newNodes, err := repairer.cache.FindStorageNodes(ctx, request)
	if err != nil {
		return Error.Wrap(err)
	}

	// Create the order limits for the PUT_REPAIR action
	putLimits, putPrivateKey, err := repairer.orders.CreatePutRepairOrderLimits(ctx, bucketID, pointer, getOrderLimits, newNodes)
	if err != nil {
		return Error.Wrap(err)
	}

	// Download the segment using just the healthy pieces
	rr, err := repairer.ec.Get(ctx, getOrderLimits, getPrivateKey, redundancy, pointer.GetSegmentSize())
	if err != nil {
		return Error.Wrap(err)
	}

	r, err := rr.Range(ctx, 0, rr.Size())
	if err != nil {
		return Error.Wrap(err)
	}
	defer func() { err = errs.Combine(err, r.Close()) }()

	// Upload the repaired pieces
	successfulNodes, hashes, err := repairer.ec.Repair(ctx, putLimits, putPrivateKey, redundancy, r, expiration, repairer.timeout, path)
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
		healthyMap[int32(i)] = true
	}

	healthyLength := int32(len(healthyPieces))
	switch {
	case healthyLength <= pointer.Remote.Redundancy.RepairThreshold:
		mon.Meter("repair_failed").Mark(1)
	case healthyLength < pointer.Remote.Redundancy.SuccessThreshold:
		mon.Meter("repair_partial").Mark(1)
	default:
		mon.Meter("repair_success").Mark(1)
	}

	healthyRatioAfterRepair := 0.0
	if pointer.Remote.Redundancy.Total != 0 {
		healthyRatioAfterRepair = float64(healthyLength) / float64(pointer.Remote.Redundancy.Total)
	}
	mon.FloatVal("healthy_ratio_after_repair").Observe(healthyRatioAfterRepair)

	// if partial repair, include "unhealthy" pieces that are not duplicates
	revisedPointerPieces := healthyPieces
	if healthyLength < pointer.Remote.Redundancy.SuccessThreshold {
		for _, p := range unhealthyPieces {
			if _, ok := healthyMap[p.GetPieceNum()]; !ok {
				revisedPointerPieces = append(revisedPointerPieces, p)
			}
		}
	}

	// Update the remote pieces in the pointer
	pointer.GetRemote().RemotePieces = revisedPointerPieces

	// Update the segment pointer in the metainfo
	return Error.Wrap(repairer.metainfo.Put(ctx, path, pointer))
}

// sliceToSet converts the given slice to a set
func sliceToSet(slice []int32) map[int32]struct{} {
	set := make(map[int32]struct{}, len(slice))
	for _, value := range slice {
		set[value] = struct{}{}
	}
	return set
}

func createBucketID(path storj.Path) ([]byte, error) {
	comps := storj.SplitPath(path)
	if len(comps) < 3 {
		return nil, Error.New("no bucket component in path: %s", path)
	}
	return []byte(storj.JoinPaths(comps[0], comps[2])), nil
}
