// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package repairer

import (
	"context"
	"math"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/rpc"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/uplink/eestream"
)

// IrreparableError is the errs class of irreparable segment errors
var IrreparableError = errs.Class("irreparable error")

// SegmentRepairer for segments
type SegmentRepairer struct {
	log      *zap.Logger
	metainfo *metainfo.Service
	orders   *orders.Service
	overlay  *overlay.Service
	ec       *ECRepairer
	timeout  time.Duration

	// multiplierOptimalThreshold is the value that multiplied by the optimal
	// threshold results in the maximum limit of number of nodes to upload
	// repaired pieces
	multiplierOptimalThreshold float64

	//repairOverride is the value handed over from the checker to override the Repair Threshold
	repairOverride int
}

// NewSegmentRepairer creates a new instance of SegmentRepairer.
//
// excessPercentageOptimalThreshold is the percentage to apply over the optimal
// threshould to determine the maximum limit of nodes to upload repaired pieces,
// when negative, 0 is applied.
func NewSegmentRepairer(
	log *zap.Logger, metainfo *metainfo.Service, orders *orders.Service,
	overlay *overlay.Service, dialer rpc.Dialer, timeout time.Duration,
	excessOptimalThreshold float64, repairOverride int,
	downloadTimeout time.Duration,
	satelliteSignee signing.Signee,
) *SegmentRepairer {

	if excessOptimalThreshold < 0 {
		excessOptimalThreshold = 0
	}

	return &SegmentRepairer{
		log:                        log,
		metainfo:                   metainfo,
		orders:                     orders,
		overlay:                    overlay,
		ec:                         NewECRepairer(log.Named("ec repairer"), dialer, satelliteSignee, downloadTimeout),
		timeout:                    timeout,
		multiplierOptimalThreshold: 1 + excessOptimalThreshold,
		repairOverride:             repairOverride,
	}
}

// Repair retrieves an at-risk segment and repairs and stores lost pieces on new nodes
// note that shouldDelete is used even in the case where err is not null
// note that it will update audit status as failed for nodes that failed piece hash verification during repair downloading
func (repairer *SegmentRepairer) Repair(ctx context.Context, path storj.Path) (shouldDelete bool, err error) {
	defer mon.Task()(&ctx, path)(&err)

	// Read the segment pointer from the metainfo
	pointer, err := repairer.metainfo.Get(ctx, path)
	if err != nil {
		if storj.ErrObjectNotFound.Has(err) {
			mon.Meter("repair_unnecessary").Mark(1)
			repairer.log.Debug("segment was deleted", zap.Binary("Segment", []byte(path)))
			return true, nil
		}
		return false, Error.Wrap(err)
	}

	if pointer.GetType() != pb.Pointer_REMOTE {
		return true, Error.New("cannot repair inline segment")
	}

	mon.Meter("repair_attempts").Mark(1)
	mon.IntVal("repair_segment_size").Observe(pointer.GetSegmentSize())

	redundancy, err := eestream.NewRedundancyStrategyFromProto(pointer.GetRemote().GetRedundancy())
	if err != nil {
		return true, Error.Wrap(err)
	}

	pieceSize := eestream.CalcPieceSize(pointer.GetSegmentSize(), redundancy)

	var excludeNodeIDs storj.NodeIDList
	var healthyPieces, unhealthyPieces []*pb.RemotePiece
	healthyMap := make(map[int32]bool)
	pieces := pointer.GetRemote().GetRemotePieces()
	missingPieces, err := repairer.overlay.GetMissingPieces(ctx, pieces)
	if err != nil {
		return false, Error.New("error getting missing pieces %s", err)
	}

	numHealthy := len(pieces) - len(missingPieces)
	// irreparable piece
	if int32(numHealthy) < pointer.Remote.Redundancy.MinReq {
		mon.Meter("repair_nodes_unavailable").Mark(1)
		return true, Error.Wrap(IrreparableError.New("segment cannot be repaired: only %d healthy pieces, %d required", numHealthy, pointer.Remote.Redundancy.MinReq+1))
	}

	repairThreshold := pointer.Remote.Redundancy.RepairThreshold
	if repairer.repairOverride != 0 {
		repairThreshold = int32(repairer.repairOverride)
	}

	// repair not needed
	if int32(numHealthy) > repairThreshold {
		mon.Meter("repair_unnecessary").Mark(1)
		repairer.log.Debug("segment above repair threshold", zap.Int("numHealthy", numHealthy), zap.Int32("repairThreshold", repairThreshold))
		return true, nil
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
		if !lostPiecesSet[piece.GetPieceNum()] {
			healthyPieces = append(healthyPieces, piece)
			healthyMap[piece.GetPieceNum()] = true
		} else {
			unhealthyPieces = append(unhealthyPieces, piece)
		}
	}

	bucketID, err := createBucketID(path)
	if err != nil {
		return true, Error.Wrap(err)
	}

	// Create the order limits for the GET_REPAIR action
	getOrderLimits, getPrivateKey, err := repairer.orders.CreateGetRepairOrderLimits(ctx, bucketID, pointer, healthyPieces)
	if err != nil {
		return false, Error.Wrap(err)
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
	newNodes, err := repairer.overlay.FindStorageNodes(ctx, request)
	if err != nil {
		return false, Error.Wrap(err)
	}

	// Create the order limits for the PUT_REPAIR action
	putLimits, putPrivateKey, err := repairer.orders.CreatePutRepairOrderLimits(ctx, bucketID, pointer, getOrderLimits, newNodes)
	if err != nil {
		return false, Error.Wrap(err)
	}

	// Download the segment using just the healthy pieces
	segmentReader, failedPieces, err := repairer.ec.Get(ctx, getOrderLimits, getPrivateKey, redundancy, pointer.GetSegmentSize(), path)

	// Populate node IDs that failed piece hashes verification
	var failedNodeIDs storj.NodeIDList
	for _, piece := range failedPieces {
		failedNodeIDs = append(failedNodeIDs, piece.NodeId)
	}

	// update audit status for nodes that failed piece hash verification during downloading
	failedNum, updateErr := repairer.updateAuditFailStatus(ctx, failedNodeIDs)
	if updateErr != nil || failedNum > 0 {
		// failed updates should not affect repair, therefore we will not return the error
		repairer.log.Debug("failed to update audit fail status", zap.Int("Failed Update Number", failedNum), zap.Error(err))
	}
	if err != nil {
		// .Get() seems to only fail from input validation, so it would keep failing
		return true, Error.Wrap(err)
	}
	defer func() { err = errs.Combine(err, segmentReader.Close()) }()

	// Upload the repaired pieces
	successfulNodes, hashes, err := repairer.ec.Repair(ctx, putLimits, putPrivateKey, redundancy, segmentReader, repairer.timeout, path)
	if err != nil {
		return false, Error.Wrap(err)
	}

	// Add the successfully uploaded pieces to repairedPieces
	var repairedPieces []*pb.RemotePiece
	repairedMap := make(map[int32]bool)
	for i, node := range successfulNodes {
		if node == nil {
			continue
		}
		piece := pb.RemotePiece{
			PieceNum: int32(i),
			NodeId:   node.Id,
			Hash:     hashes[i],
		}
		repairedPieces = append(repairedPieces, &piece)
		repairedMap[int32(i)] = true
	}

	healthyAfterRepair := int32(len(healthyPieces) + len(repairedPieces))
	switch {
	case healthyAfterRepair <= pointer.Remote.Redundancy.RepairThreshold:
		mon.Meter("repair_failed").Mark(1)
	case healthyAfterRepair < pointer.Remote.Redundancy.SuccessThreshold:
		mon.Meter("repair_partial").Mark(1)
	default:
		mon.Meter("repair_success").Mark(1)
	}

	healthyRatioAfterRepair := 0.0
	if pointer.Remote.Redundancy.Total != 0 {
		healthyRatioAfterRepair = float64(healthyAfterRepair) / float64(pointer.Remote.Redundancy.Total)
	}
	mon.FloatVal("healthy_ratio_after_repair").Observe(healthyRatioAfterRepair)

	var toRemove []*pb.RemotePiece
	if healthyAfterRepair >= pointer.Remote.Redundancy.SuccessThreshold {
		// if full repair, remove all unhealthy pieces
		toRemove = unhealthyPieces
	} else {
		// if partial repair, leave unrepaired unhealthy pieces in the pointer
		for _, piece := range unhealthyPieces {
			if repairedMap[piece.GetPieceNum()] {
				// add only repaired pieces in the slice, unrepaired
				// unhealthy pieces are not removed from the pointer
				toRemove = append(toRemove, piece)
			}
		}
	}

	// add pieces that failed piece hashes verification to the removal list
	toRemove = append(toRemove, failedPieces...)

	var segmentAge time.Duration
	if pointer.CreationDate.Before(pointer.LastRepaired) {
		segmentAge = time.Since(pointer.LastRepaired)
	} else {
		segmentAge = time.Since(pointer.CreationDate)
	}

	pointer.LastRepaired = time.Now().UTC()
	pointer.RepairCount++

	// Update the segment pointer in the metainfo
	_, err = repairer.metainfo.UpdatePieces(ctx, path, pointer, repairedPieces, toRemove)
	if err != nil {
		return false, err
	}

	mon.IntVal("segment_time_until_repair").Observe(int64(segmentAge.Seconds()))
	mon.IntVal("segment_repair_count").Observe(int64(pointer.RepairCount))

	return true, nil
}

func (repairer *SegmentRepairer) updateAuditFailStatus(ctx context.Context, failedAuditNodeIDs storj.NodeIDList) (failedNum int, err error) {
	updateRequests := make([]*overlay.UpdateRequest, len(failedAuditNodeIDs))
	for i, nodeID := range failedAuditNodeIDs {
		updateRequests[i] = &overlay.UpdateRequest{
			NodeID:       nodeID,
			IsUp:         true,
			AuditSuccess: false,
		}
	}
	if len(updateRequests) > 0 {
		failed, err := repairer.overlay.BatchUpdateStats(ctx, updateRequests)
		if err != nil || len(failed) > 0 {
			return len(failed), errs.Combine(Error.New("failed to update some audit fail statuses in overlay"), err)
		}
	}
	return 0, nil
}

// sliceToSet converts the given slice to a set
func sliceToSet(slice []int32) map[int32]bool {
	set := make(map[int32]bool, len(slice))
	for _, value := range slice {
		set[value] = true
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
