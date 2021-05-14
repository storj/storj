// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package repairer

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/rpc"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/repair/checker"
	"storj.io/uplink/private/eestream"
)

var (
	metainfoGetError       = errs.Class("metainfo db get")
	metainfoPutError       = errs.Class("metainfo db put")
	invalidRepairError     = errs.Class("invalid repair")
	overlayQueryError      = errs.Class("overlay query failure")
	orderLimitFailureError = errs.Class("order limits failure")
	repairReconstructError = errs.Class("repair reconstruction failure")
	repairPutError         = errs.Class("repair could not store repaired pieces")
)

// irreparableError identifies situations where a segment could not be repaired due to reasons
// which are hopefully transient (e.g. too many pieces unavailable). The segment should be added
// to the irreparableDB.
type irreparableError struct {
	path            storj.Path
	piecesAvailable int32
	piecesRequired  int32
}

func (ie *irreparableError) Error() string {
	return fmt.Sprintf("%d available pieces < %d required", ie.piecesAvailable, ie.piecesRequired)
}

// SegmentRepairer for segments.
type SegmentRepairer struct {
	log            *zap.Logger
	statsCollector *statsCollector
	metabase       *metabase.DB
	orders         *orders.Service
	overlay        *overlay.Service
	ec             *ECRepairer
	timeout        time.Duration

	// multiplierOptimalThreshold is the value that multiplied by the optimal
	// threshold results in the maximum limit of number of nodes to upload
	// repaired pieces
	multiplierOptimalThreshold float64

	// repairOverrides is the set of values configured by the checker to override the repair threshold for various RS schemes.
	repairOverrides checker.RepairOverridesMap

	nowFn func() time.Time
}

// NewSegmentRepairer creates a new instance of SegmentRepairer.
//
// excessPercentageOptimalThreshold is the percentage to apply over the optimal
// threshould to determine the maximum limit of nodes to upload repaired pieces,
// when negative, 0 is applied.
func NewSegmentRepairer(
	log *zap.Logger, metabase *metabase.DB, orders *orders.Service,
	overlay *overlay.Service, dialer rpc.Dialer, timeout time.Duration,
	excessOptimalThreshold float64, repairOverrides checker.RepairOverrides,
	downloadTimeout time.Duration, inMemoryRepair bool,
	satelliteSignee signing.Signee,
) *SegmentRepairer {

	if excessOptimalThreshold < 0 {
		excessOptimalThreshold = 0
	}

	return &SegmentRepairer{
		log:                        log,
		statsCollector:             newStatsCollector(),
		metabase:                   metabase,
		orders:                     orders,
		overlay:                    overlay,
		ec:                         NewECRepairer(log.Named("ec repairer"), dialer, satelliteSignee, downloadTimeout, inMemoryRepair),
		timeout:                    timeout,
		multiplierOptimalThreshold: 1 + excessOptimalThreshold,
		repairOverrides:            repairOverrides.GetMap(),

		nowFn: time.Now,
	}
}

// Repair retrieves an at-risk segment and repairs and stores lost pieces on new nodes
// note that shouldDelete is used even in the case where err is not null
// note that it will update audit status as failed for nodes that failed piece hash verification during repair downloading.
func (repairer *SegmentRepairer) Repair(ctx context.Context, path storj.Path) (shouldDelete bool, err error) {
	defer mon.Task()(&ctx, path)(&err)

	// TODO extend InjuredSegment with StreamID/Position and replace path
	segmentLocation, err := metabase.ParseSegmentKey(metabase.SegmentKey(path))
	if err != nil {
		return false, metainfoGetError.Wrap(err)
	}

	// TODO we should replace GetSegmentByLocation with GetSegmentByPosition when
	// we refactor the repair queue to store metabase.SegmentPosition instead of storj.Path.
	segment, err := repairer.metabase.GetSegmentByLocation(ctx, metabase.GetSegmentByLocation{
		SegmentLocation: segmentLocation,
	})
	if err != nil {
		if storj.ErrObjectNotFound.Has(err) {
			mon.Meter("repair_unnecessary").Mark(1)            //mon:locked
			mon.Meter("segment_deleted_before_repair").Mark(1) //mon:locked
			repairer.log.Debug("segment was deleted")
			return true, nil
		}
		return false, metainfoGetError.Wrap(err)
	}

	if segment.Inline() {
		return true, invalidRepairError.New("cannot repair inline segment")
	}

	redundancy, err := eestream.NewRedundancyStrategyFromStorj(segment.Redundancy)
	if err != nil {
		return true, invalidRepairError.New("invalid redundancy strategy: %w", err)
	}

	stats := repairer.getStatsByRS(&pb.RedundancyScheme{
		Type:             pb.RedundancyScheme_SchemeType(segment.Redundancy.Algorithm),
		ErasureShareSize: segment.Redundancy.ShareSize,
		MinReq:           int32(segment.Redundancy.RequiredShares),
		RepairThreshold:  int32(segment.Redundancy.RepairShares),
		SuccessThreshold: int32(segment.Redundancy.OptimalShares),
		Total:            int32(segment.Redundancy.TotalShares),
	})

	mon.Meter("repair_attempts").Mark(1) //mon:locked
	stats.repairAttempts.Mark(1)
	mon.IntVal("repair_segment_size").Observe(int64(segment.EncryptedSize)) //mon:locked
	stats.repairSegmentSize.Observe(int64(segment.EncryptedSize))

	var excludeNodeIDs storj.NodeIDList
	pieces := segment.Pieces
	missingPieces, err := repairer.overlay.GetMissingPieces(ctx, pieces)
	if err != nil {
		return false, overlayQueryError.New("error identifying missing pieces: %w", err)
	}

	numHealthy := len(pieces) - len(missingPieces)
	// irreparable piece
	if numHealthy < int(segment.Redundancy.RequiredShares) {
		mon.Counter("repairer_segments_below_min_req").Inc(1) //mon:locked
		stats.repairerSegmentsBelowMinReq.Inc(1)
		mon.Meter("repair_nodes_unavailable").Mark(1) //mon:locked
		stats.repairerNodesUnavailable.Mark(1)
		return true, &irreparableError{
			path:            path,
			piecesAvailable: int32(numHealthy),
			piecesRequired:  int32(segment.Redundancy.RequiredShares),
		}
	}

	// ensure we get values, even if only zero values, so that redash can have an alert based on this
	mon.Counter("repairer_segments_below_min_req").Inc(0) //mon:locked
	stats.repairerSegmentsBelowMinReq.Inc(0)

	repairThreshold := int32(segment.Redundancy.RepairShares)

	pbRedundancy := &pb.RedundancyScheme{
		MinReq:           int32(segment.Redundancy.RequiredShares),
		RepairThreshold:  int32(segment.Redundancy.RepairShares),
		SuccessThreshold: int32(segment.Redundancy.OptimalShares),
		Total:            int32(segment.Redundancy.TotalShares),
	}
	overrideValue := repairer.repairOverrides.GetOverrideValuePB(pbRedundancy)
	if overrideValue != 0 {
		repairThreshold = overrideValue
	}

	// repair not needed
	if numHealthy > int(repairThreshold) {
		mon.Meter("repair_unnecessary").Mark(1) //mon:locked
		stats.repairUnnecessary.Mark(1)
		repairer.log.Debug("segment above repair threshold", zap.Int("numHealthy", numHealthy), zap.Int32("repairThreshold", repairThreshold))
		return true, nil
	}

	healthyRatioBeforeRepair := 0.0
	if segment.Redundancy.TotalShares != 0 {
		healthyRatioBeforeRepair = float64(numHealthy) / float64(segment.Redundancy.TotalShares)
	}
	mon.FloatVal("healthy_ratio_before_repair").Observe(healthyRatioBeforeRepair) //mon:locked
	stats.healthyRatioBeforeRepair.Observe(healthyRatioBeforeRepair)

	lostPiecesSet := sliceToSet(missingPieces)

	var healthyPieces, unhealthyPieces metabase.Pieces
	healthyMap := make(map[uint16]bool)
	// Populate healthyPieces with all pieces from the segment except those correlating to indices in lostPieces
	for _, piece := range pieces {
		excludeNodeIDs = append(excludeNodeIDs, piece.StorageNode)
		if !lostPiecesSet[piece.Number] {
			healthyPieces = append(healthyPieces, piece)
			healthyMap[piece.Number] = true
		} else {
			unhealthyPieces = append(unhealthyPieces, piece)
		}
	}

	bucket := segmentLocation.Bucket()

	// Create the order limits for the GET_REPAIR action
	getOrderLimits, getPrivateKey, err := repairer.orders.CreateGetRepairOrderLimits(ctx, bucket, segment, healthyPieces)
	if err != nil {
		return false, orderLimitFailureError.New("could not create GET_REPAIR order limits: %w", err)
	}

	// Double check for healthy pieces which became unhealthy inside CreateGetRepairOrderLimits
	// Remove them from healthyPieces and add them to unhealthyPieces
	var newHealthyPieces metabase.Pieces
	for _, piece := range healthyPieces {
		if getOrderLimits[piece.Number] == nil {
			unhealthyPieces = append(unhealthyPieces, piece)
		} else {
			newHealthyPieces = append(newHealthyPieces, piece)
		}
	}
	healthyPieces = newHealthyPieces

	var requestCount int
	var minSuccessfulNeeded int
	{
		totalNeeded := math.Ceil(float64(redundancy.OptimalThreshold()) * repairer.multiplierOptimalThreshold)
		requestCount = int(totalNeeded) - len(healthyPieces)
		minSuccessfulNeeded = redundancy.OptimalThreshold() - len(healthyPieces)
	}

	// Request Overlay for n-h new storage nodes
	request := overlay.FindStorageNodesRequest{
		RequestedCount: requestCount,
		ExcludedIDs:    excludeNodeIDs,
	}
	newNodes, err := repairer.overlay.FindStorageNodesForUpload(ctx, request)
	if err != nil {
		return false, overlayQueryError.Wrap(err)
	}

	// Create the order limits for the PUT_REPAIR action
	putLimits, putPrivateKey, err := repairer.orders.CreatePutRepairOrderLimits(ctx, bucket, segment, getOrderLimits, newNodes, repairer.multiplierOptimalThreshold)
	if err != nil {
		return false, orderLimitFailureError.New("could not create PUT_REPAIR order limits: %w", err)
	}

	// Download the segment using just the healthy pieces
	segmentReader, pbFailedPieces, err := repairer.ec.Get(ctx, getOrderLimits, getPrivateKey, redundancy, int64(segment.EncryptedSize), path)

	// Populate node IDs that failed piece hashes verification
	var failedNodeIDs storj.NodeIDList
	for _, piece := range pbFailedPieces {
		failedNodeIDs = append(failedNodeIDs, piece.NodeId)
	}

	// TODO refactor repairer.ec.Get?
	failedPieces := make(metabase.Pieces, len(pbFailedPieces))
	for i, piece := range pbFailedPieces {
		failedPieces[i] = metabase.Piece{
			Number:      uint16(piece.PieceNum),
			StorageNode: piece.NodeId,
		}
	}

	// update audit status for nodes that failed piece hash verification during downloading
	failedNum, updateErr := repairer.updateAuditFailStatus(ctx, failedNodeIDs)
	if updateErr != nil || failedNum > 0 {
		// failed updates should not affect repair, therefore we will not return the error
		repairer.log.Debug("failed to update audit fail status", zap.Int("Failed Update Number", failedNum), zap.Error(updateErr))
	}
	if err != nil {
		// If the context was closed during the Get phase, it will appear here as though
		// we just failed to download enough pieces to reconstruct the segment. Check for
		// a closed context before doing any further error processing.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return false, ctxErr
		}
		// If Get failed because of input validation, then it will keep failing. But if it
		// gave us irreparableError, then we failed to download enough pieces and must try
		// to wait for nodes to come back online.
		var irreparableErr *irreparableError
		if errors.As(err, &irreparableErr) {
			mon.Meter("repair_too_many_nodes_failed").Mark(1) //mon:locked
			stats.repairTooManyNodesFailed.Mark(1)
			// irreparableErr.segmentInfo = pointer
			return true, irreparableErr
		}
		// The segment's redundancy strategy is invalid, or else there was an internal error.
		return true, repairReconstructError.New("segment could not be reconstructed: %w", err)
	}
	defer func() { err = errs.Combine(err, segmentReader.Close()) }()

	// Upload the repaired pieces
	successfulNodes, _, err := repairer.ec.Repair(ctx, putLimits, putPrivateKey, redundancy, segmentReader, repairer.timeout, path, minSuccessfulNeeded)
	if err != nil {
		return false, repairPutError.Wrap(err)
	}

	// Add the successfully uploaded pieces to repairedPieces
	var repairedPieces metabase.Pieces
	repairedMap := make(map[uint16]bool)
	for i, node := range successfulNodes {
		if node == nil {
			continue
		}
		piece := metabase.Piece{
			Number:      uint16(i),
			StorageNode: node.Id,
		}
		repairedPieces = append(repairedPieces, piece)
		repairedMap[uint16(i)] = true
	}

	healthyAfterRepair := len(healthyPieces) + len(repairedPieces)
	switch {
	case healthyAfterRepair <= int(segment.Redundancy.RepairShares):
		// Important: this indicates a failure to PUT enough pieces to the network to pass
		// the repair threshold, and _not_ a failure to reconstruct the segment. But we
		// put at least one piece, else ec.Repair() would have returned an error. So the
		// repair "succeeded" in that the segment is now healthier than it was, but it is
		// not as healthy as we want it to be.
		mon.Meter("repair_failed").Mark(1) //mon:locked
		stats.repairFailed.Mark(1)
	case healthyAfterRepair < int(segment.Redundancy.OptimalShares):
		mon.Meter("repair_partial").Mark(1) //mon:locked
		stats.repairPartial.Mark(1)
	default:
		mon.Meter("repair_success").Mark(1) //mon:locked
		stats.repairSuccess.Mark(1)
	}

	healthyRatioAfterRepair := 0.0
	if segment.Redundancy.TotalShares != 0 {
		healthyRatioAfterRepair = float64(healthyAfterRepair) / float64(segment.Redundancy.TotalShares)
	}

	mon.FloatVal("healthy_ratio_after_repair").Observe(healthyRatioAfterRepair) //mon:locked
	stats.healthyRatioAfterRepair.Observe(healthyRatioAfterRepair)

	var toRemove metabase.Pieces
	if healthyAfterRepair >= int(segment.Redundancy.OptimalShares) {
		// if full repair, remove all unhealthy pieces
		toRemove = unhealthyPieces
	} else {
		// if partial repair, leave unrepaired unhealthy pieces in the pointer
		for _, piece := range unhealthyPieces {
			if repairedMap[piece.Number] {
				// add only repaired pieces in the slice, unrepaired
				// unhealthy pieces are not removed from the pointer
				toRemove = append(toRemove, piece)
			}
		}
	}

	// add pieces that failed piece hashes verification to the removal list
	toRemove = append(toRemove, failedPieces...)

	newPieces, err := updatePieces(segment.Pieces, repairedPieces, toRemove)
	if err != nil {
		return false, repairPutError.Wrap(err)
	}

	err = repairer.metabase.UpdateSegmentPieces(ctx, metabase.UpdateSegmentPieces{
		StreamID: segment.StreamID,
		Position: segmentLocation.Position,

		OldPieces:     segment.Pieces,
		NewRedundancy: segment.Redundancy,
		NewPieces:     newPieces,

		NewRepairedAt: time.Now(),
	})
	if err != nil {
		return false, metainfoPutError.Wrap(err)
	}

	createdAt := time.Time{}
	if segment.CreatedAt != nil {
		createdAt = *segment.CreatedAt
	}
	repairedAt := time.Time{}
	if segment.RepairedAt != nil {
		repairedAt = *segment.RepairedAt
	}

	var segmentAge time.Duration
	if createdAt.Before(repairedAt) {
		segmentAge = time.Since(repairedAt)
	} else {
		segmentAge = time.Since(createdAt)
	}

	// TODO what to do with RepairCount
	var repairCount int64
	// pointer.RepairCount++

	mon.IntVal("segment_time_until_repair").Observe(int64(segmentAge.Seconds())) //mon:locked
	stats.segmentTimeUntilRepair.Observe((int64(segmentAge.Seconds())))
	mon.IntVal("segment_repair_count").Observe(repairCount) //mon:locked
	stats.segmentRepairCount.Observe(repairCount)

	return true, nil
}

func updatePieces(orignalPieces, toAddPieces, toRemovePieces metabase.Pieces) (metabase.Pieces, error) {
	pieceMap := make(map[uint16]metabase.Piece)
	for _, piece := range orignalPieces {
		pieceMap[piece.Number] = piece
	}

	// remove the toRemove pieces from the map
	// only if all piece number, node id match
	for _, piece := range toRemovePieces {
		if piece == (metabase.Piece{}) {
			continue
		}
		existing := pieceMap[piece.Number]
		if existing != (metabase.Piece{}) && existing.StorageNode == piece.StorageNode {
			delete(pieceMap, piece.Number)
		}
	}

	// add the pieces to the map
	for _, piece := range toAddPieces {
		if piece == (metabase.Piece{}) {
			continue
		}
		_, exists := pieceMap[piece.Number]
		if exists {
			return metabase.Pieces{}, Error.New("piece to add already exists (piece no: %d)", piece.Number)
		}
		pieceMap[piece.Number] = piece
	}

	newPieces := make(metabase.Pieces, 0, len(pieceMap))
	for _, piece := range pieceMap {
		newPieces = append(newPieces, piece)
	}
	sort.Sort(newPieces)

	return newPieces, nil
}

func (repairer *SegmentRepairer) getStatsByRS(redundancy *pb.RedundancyScheme) *stats {
	rsString := getRSString(repairer.loadRedundancy(redundancy))
	return repairer.statsCollector.getStatsByRS(rsString)
}

func (repairer *SegmentRepairer) loadRedundancy(redundancy *pb.RedundancyScheme) (int, int, int, int) {
	repair := int(redundancy.RepairThreshold)
	overrideValue := repairer.repairOverrides.GetOverrideValuePB(redundancy)
	if overrideValue != 0 {
		repair = int(overrideValue)
	}
	return int(redundancy.MinReq), repair, int(redundancy.SuccessThreshold), int(redundancy.Total)
}

func (repairer *SegmentRepairer) updateAuditFailStatus(ctx context.Context, failedAuditNodeIDs storj.NodeIDList) (failedNum int, err error) {
	updateRequests := make([]*overlay.UpdateRequest, len(failedAuditNodeIDs))
	for i, nodeID := range failedAuditNodeIDs {
		updateRequests[i] = &overlay.UpdateRequest{
			NodeID:       nodeID,
			AuditOutcome: overlay.AuditFailure,
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

// SetNow allows tests to have the server act as if the current time is whatever they want.
func (repairer *SegmentRepairer) SetNow(nowFn func() time.Time) {
	repairer.nowFn = nowFn
}

// sliceToSet converts the given slice to a set.
func sliceToSet(slice []uint16) map[uint16]bool {
	set := make(map[uint16]bool, len(slice))
	for _, value := range slice {
		set[value] = true
	}
	return set
}
