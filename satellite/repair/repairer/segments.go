// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package repairer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/exp/maps"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/storj/location"
	"storj.io/common/sync2"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/repair"
	"storj.io/storj/satellite/repair/checker"
	"storj.io/storj/satellite/repair/queue"
	"storj.io/uplink/private/eestream"
	"storj.io/uplink/private/piecestore"
)

var (
	metainfoGetError       = errs.Class("metainfo db get")
	metainfoPutError       = errs.Class("metainfo db put")
	invalidRepairError     = errs.Class("invalid repair")
	overlayQueryError      = errs.Class("overlay query failure")
	orderLimitFailureError = errs.Class("order limits failure")
	repairReconstructError = errs.Class("repair reconstruction failure")
	repairPutError         = errs.Class("repair could not store repaired pieces")
	// segmentVerificationError is the errs class when the repaired segment can not be verified during repair.
	segmentVerificationError = errs.Class("segment verification failed")
	// segmentDeletedError is the errs class when the repaired segment was deleted during the repair.
	segmentDeletedError = errs.Class("segment deleted during repair")
	// segmentModifiedError is the errs class used when a segment has been changed in any way.
	segmentModifiedError = errs.Class("segment has been modified")
)

// irreparableError identifies situations where a segment could not be repaired due to reasons
// which are hopefully transient (e.g. too many pieces unavailable). The segment should be added
// to the irreparableDB.
type irreparableError struct {
	piecesAvailable int32
	piecesRequired  int32
}

func (ie *irreparableError) Error() string {
	return fmt.Sprintf("%d available pieces < %d required", ie.piecesAvailable, ie.piecesRequired)
}

// PieceFetchResult combines a piece pointer with the error we got when we tried
// to acquire that piece.
type PieceFetchResult struct {
	Piece metabase.Piece
	Err   error
}

// FetchResultReport contains a categorization of a set of pieces based on the results of
// GET operations.
type FetchResultReport struct {
	Successful []PieceFetchResult
	Failed     []PieceFetchResult
	Offline    []PieceFetchResult
	Contained  []PieceFetchResult
	Unknown    []PieceFetchResult
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
	reporter       audit.Reporter

	reputationUpdateEnabled bool
	doDeclumping            bool
	doPlacementCheck        bool

	// multiplierOptimalThreshold is the value that multiplied by the optimal
	// threshold results in the maximum limit of number of nodes to upload
	// repaired pieces
	multiplierOptimalThreshold float64

	// repairOverrides is the set of values configured by the checker to override the repair threshold for various RS schemes.
	repairOverrides checker.RepairOverridesMap

	excludedCountryCodes map[location.CountryCode]struct{}

	nowFn                            func() time.Time
	OnTestingCheckSegmentAlteredHook func()
	OnTestingPiecesReportHook        func(pieces FetchResultReport)
	placementRules                   overlay.PlacementRules
}

// NewSegmentRepairer creates a new instance of SegmentRepairer.
//
// excessPercentageOptimalThreshold is the percentage to apply over the optimal
// threshould to determine the maximum limit of nodes to upload repaired pieces,
// when negative, 0 is applied.
func NewSegmentRepairer(
	log *zap.Logger,
	metabase *metabase.DB,
	orders *orders.Service,
	overlay *overlay.Service,
	reporter audit.Reporter,
	ecRepairer *ECRepairer,
	placementRules overlay.PlacementRules,
	repairOverrides checker.RepairOverrides,
	config Config,
) *SegmentRepairer {

	excessOptimalThreshold := config.MaxExcessRateOptimalThreshold
	if excessOptimalThreshold < 0 {
		excessOptimalThreshold = 0
	}

	excludedCountryCodes := make(map[location.CountryCode]struct{})
	for _, countryCode := range config.RepairExcludedCountryCodes {
		if cc := location.ToCountryCode(countryCode); cc != location.None {
			excludedCountryCodes[cc] = struct{}{}
		}
	}

	return &SegmentRepairer{
		log:                        log,
		statsCollector:             newStatsCollector(),
		metabase:                   metabase,
		orders:                     orders,
		overlay:                    overlay,
		ec:                         ecRepairer,
		timeout:                    config.Timeout,
		multiplierOptimalThreshold: 1 + excessOptimalThreshold,
		repairOverrides:            repairOverrides.GetMap(),
		excludedCountryCodes:       excludedCountryCodes,
		reporter:                   reporter,
		reputationUpdateEnabled:    config.ReputationUpdateEnabled,
		doDeclumping:               config.DoDeclumping,
		doPlacementCheck:           config.DoPlacementCheck,
		placementRules:             placementRules,

		nowFn: time.Now,
	}
}

// Repair retrieves an at-risk segment and repairs and stores lost pieces on new nodes
// note that shouldDelete is used even in the case where err is not null
// note that it will update audit status as failed for nodes that failed piece hash verification during repair downloading.
func (repairer *SegmentRepairer) Repair(ctx context.Context, queueSegment *queue.InjuredSegment) (shouldDelete bool, err error) {
	defer mon.Task()(&ctx, queueSegment.StreamID.String(), queueSegment.Position.Encode())(&err)

	segment, err := repairer.metabase.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
		StreamID: queueSegment.StreamID,
		Position: queueSegment.Position,
	})
	if err != nil {
		if metabase.ErrSegmentNotFound.Has(err) {
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

	// ignore segment if expired
	if segment.Expired(repairer.nowFn()) {
		mon.Meter("repair_unnecessary").Mark(1)
		mon.Meter("segment_expired_before_repair").Mark(1)
		repairer.log.Debug("segment has expired", zap.Stringer("Stream ID", segment.StreamID), zap.Uint64("Position", queueSegment.Position.Encode()))
		return true, nil
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

	piecesCheck, err := repairer.classifySegmentPieces(ctx, segment)
	if err != nil {
		return false, err
	}

	pieces := segment.Pieces

	numRetrievable := len(pieces) - len(piecesCheck.MissingPiecesSet)
	numHealthy := len(pieces) - len(piecesCheck.MissingPiecesSet) - piecesCheck.NumUnhealthyRetrievable
	// irreparable segment
	if numRetrievable < int(segment.Redundancy.RequiredShares) {
		mon.Counter("repairer_segments_below_min_req").Inc(1) //mon:locked
		stats.repairerSegmentsBelowMinReq.Inc(1)
		mon.Meter("repair_nodes_unavailable").Mark(1) //mon:locked
		stats.repairerNodesUnavailable.Mark(1)

		repairer.log.Warn("irreparable segment",
			zap.String("StreamID", queueSegment.StreamID.String()),
			zap.Uint64("Position", queueSegment.Position.Encode()),
			zap.Int("piecesAvailable", numRetrievable),
			zap.Int16("piecesRequired", segment.Redundancy.RequiredShares),
		)
		return false, nil
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
	if numHealthy-piecesCheck.NumHealthyInExcludedCountries > int(repairThreshold) {
		// remove pieces out of placement without repairing as we are above repair threshold
		if len(piecesCheck.OutOfPlacementPiecesSet) > 0 {

			var outOfPlacementPieces metabase.Pieces
			for _, piece := range pieces {
				if _, ok := piecesCheck.OutOfPlacementPiecesSet[piece.Number]; ok {
					outOfPlacementPieces = append(outOfPlacementPieces, piece)
				}
			}

			newPieces, err := segment.Pieces.Update(nil, outOfPlacementPieces)
			if err != nil {
				return false, metainfoPutError.Wrap(err)
			}

			err = repairer.metabase.UpdateSegmentPieces(ctx, metabase.UpdateSegmentPieces{
				StreamID: segment.StreamID,
				Position: segment.Position,

				OldPieces:     segment.Pieces,
				NewRedundancy: segment.Redundancy,
				NewPieces:     newPieces,

				NewRepairedAt: time.Now(),
			})
			if err != nil {
				return false, metainfoPutError.Wrap(err)
			}

			mon.Meter("dropped_out_of_placement_pieces").Mark(len(piecesCheck.OutOfPlacementPiecesSet))
		}

		mon.Meter("repair_unnecessary").Mark(1) //mon:locked
		stats.repairUnnecessary.Mark(1)
		repairer.log.Debug("segment above repair threshold", zap.Int("numHealthy", numHealthy), zap.Int32("repairThreshold", repairThreshold),
			zap.Int("numClumped", len(piecesCheck.ClumpedPiecesSet)), zap.Int("numOffPieces", len(piecesCheck.OutOfPlacementPiecesSet)))
		return true, nil
	}

	healthyRatioBeforeRepair := 0.0
	if segment.Redundancy.TotalShares != 0 {
		healthyRatioBeforeRepair = float64(numHealthy) / float64(segment.Redundancy.TotalShares)
	}
	mon.FloatVal("healthy_ratio_before_repair").Observe(healthyRatioBeforeRepair) //mon:locked
	stats.healthyRatioBeforeRepair.Observe(healthyRatioBeforeRepair)

	lostPiecesSet := piecesCheck.MissingPiecesSet

	var retrievablePieces metabase.Pieces
	unhealthyPieces := make(map[metabase.Piece]struct{})
	healthySet := make(map[int32]struct{})
	// Populate retrievablePieces with all pieces from the segment except those correlating to indices in lostPieces.
	// Populate unhealthyPieces with all pieces in lostPieces, clumpedPieces or outOfPlacementPieces.
	for _, piece := range pieces {
		if lostPiecesSet[piece.Number] {
			unhealthyPieces[piece] = struct{}{}
		} else {
			retrievablePieces = append(retrievablePieces, piece)
			if piecesCheck.ClumpedPiecesSet[piece.Number] || piecesCheck.OutOfPlacementPiecesSet[piece.Number] {
				unhealthyPieces[piece] = struct{}{}
			} else {
				healthySet[int32(piece.Number)] = struct{}{}
			}
		}
	}

	// Create the order limits for the GET_REPAIR action
	getOrderLimits, getPrivateKey, cachedNodesInfo, err := repairer.orders.CreateGetRepairOrderLimits(ctx, segment, retrievablePieces)
	if err != nil {
		if orders.ErrDownloadFailedNotEnoughPieces.Has(err) {
			mon.Counter("repairer_segments_below_min_req").Inc(1) //mon:locked
			stats.repairerSegmentsBelowMinReq.Inc(1)
			mon.Meter("repair_nodes_unavailable").Mark(1) //mon:locked
			stats.repairerNodesUnavailable.Mark(1)

			repairer.log.Warn("irreparable segment: too many nodes offline",
				zap.String("StreamID", queueSegment.StreamID.String()),
				zap.Uint64("Position", queueSegment.Position.Encode()),
				zap.Int("piecesAvailable", len(retrievablePieces)),
				zap.Int16("piecesRequired", segment.Redundancy.RequiredShares),
				zap.Error(err),
			)
		}
		return false, orderLimitFailureError.New("could not create GET_REPAIR order limits: %w", err)
	}

	// Double check for retrievable pieces which became irretrievable inside CreateGetRepairOrderLimits
	// Add them to unhealthyPieces.
	for _, piece := range retrievablePieces {
		if getOrderLimits[piece.Number] == nil {
			unhealthyPieces[piece] = struct{}{}
		}
	}
	numHealthy = len(healthySet)

	var requestCount int
	var minSuccessfulNeeded int
	{
		totalNeeded := math.Ceil(float64(redundancy.OptimalThreshold()) * repairer.multiplierOptimalThreshold)
		requestCount = int(totalNeeded) + piecesCheck.NumHealthyInExcludedCountries
		if requestCount > redundancy.TotalCount() {
			requestCount = redundancy.TotalCount()
		}
		requestCount -= numHealthy
		minSuccessfulNeeded = redundancy.OptimalThreshold() - numHealthy + piecesCheck.NumHealthyInExcludedCountries
	}

	// Request Overlay for n-h new storage nodes
	request := overlay.FindStorageNodesRequest{
		RequestedCount: requestCount,
		ExcludedIDs:    piecesCheck.ExcludeNodeIDs,
		Placement:      segment.Placement,
	}
	newNodes, err := repairer.overlay.FindStorageNodesForUpload(ctx, request)
	if err != nil {
		return false, overlayQueryError.Wrap(err)
	}

	// Create the order limits for the PUT_REPAIR action
	putLimits, putPrivateKey, err := repairer.orders.CreatePutRepairOrderLimits(ctx, segment, getOrderLimits, healthySet, newNodes, repairer.multiplierOptimalThreshold, piecesCheck.NumHealthyInExcludedCountries)
	if err != nil {
		return false, orderLimitFailureError.New("could not create PUT_REPAIR order limits: %w", err)
	}

	// Download the segment using just the retrievable pieces
	segmentReader, piecesReport, err := repairer.ec.Get(ctx, getOrderLimits, cachedNodesInfo, getPrivateKey, redundancy, int64(segment.EncryptedSize))

	// ensure we get values, even if only zero values, so that redash can have an alert based on this
	mon.Meter("repair_too_many_nodes_failed").Mark(0)     //mon:locked
	mon.Meter("repair_suspected_network_problem").Mark(0) //mon:locked
	stats.repairTooManyNodesFailed.Mark(0)

	if repairer.OnTestingPiecesReportHook != nil {
		repairer.OnTestingPiecesReportHook(piecesReport)
	}

	// Check if segment has been altered
	checkSegmentError := repairer.checkIfSegmentAltered(ctx, segment)
	if checkSegmentError != nil {
		if segmentDeletedError.Has(checkSegmentError) {
			// mon.Meter("segment_deleted_during_repair").Mark(1) //mon:locked
			repairer.log.Debug("segment deleted during Repair")
			return true, nil
		}
		if segmentModifiedError.Has(checkSegmentError) {
			// mon.Meter("segment_modified_during_repair").Mark(1) //mon:locked
			repairer.log.Debug("segment modified during Repair")
			return true, nil
		}
		return false, segmentVerificationError.Wrap(checkSegmentError)
	}

	if len(piecesReport.Contained) > 0 {
		repairer.log.Debug("unexpected contained pieces during repair", zap.Int("count", len(piecesReport.Contained)))
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
			// piecesReport.Offline:
			// Nodes which were online recently, but which we couldn't contact for
			// this operation.
			//
			// piecesReport.Failed:
			// Nodes which we contacted successfully but which indicated they
			// didn't have the piece we wanted.
			//
			// piecesReport.Contained:
			// Nodes which we contacted successfully but timed out after we asked
			// for the piece.
			//
			// piecesReport.Unknown:
			// Something else went wrong, and we don't know what.
			//
			// In a network failure scenario, we expect more than half of the outcomes
			// will be in Offline or Contained.
			if len(piecesReport.Offline)+len(piecesReport.Contained) > len(piecesReport.Successful)+len(piecesReport.Failed)+len(piecesReport.Unknown) {
				mon.Meter("repair_suspected_network_problem").Mark(1) //mon:locked
			} else {
				mon.Meter("repair_too_many_nodes_failed").Mark(1) //mon:locked
			}
			stats.repairTooManyNodesFailed.Mark(1)

			failedNodeIDs := make([]string, 0, len(piecesReport.Failed))
			offlineNodeIDs := make([]string, 0, len(piecesReport.Offline))
			timedOutNodeIDs := make([]string, 0, len(piecesReport.Contained))
			unknownErrs := make([]string, 0, len(piecesReport.Unknown))
			for _, outcome := range piecesReport.Failed {
				failedNodeIDs = append(failedNodeIDs, outcome.Piece.StorageNode.String())
			}
			for _, outcome := range piecesReport.Offline {
				offlineNodeIDs = append(offlineNodeIDs, outcome.Piece.StorageNode.String())
			}
			for _, outcome := range piecesReport.Contained {
				timedOutNodeIDs = append(timedOutNodeIDs, outcome.Piece.StorageNode.String())
			}
			for _, outcome := range piecesReport.Unknown {
				// We are purposefully using the error's string here, as opposed
				// to wrapping the error. It is not likely that we need the local-side
				// traceback of where this error was initially wrapped, and this will
				// keep the logs more readable.
				unknownErrs = append(unknownErrs, fmt.Sprintf("node ID [%s] err: %v", outcome.Piece.StorageNode, outcome.Err))
			}

			repairer.log.Warn("irreparable segment: could not acquire enough shares",
				zap.String("StreamID", queueSegment.StreamID.String()),
				zap.Uint64("Position", queueSegment.Position.Encode()),
				zap.Int32("piecesAvailable", irreparableErr.piecesAvailable),
				zap.Int32("piecesRequired", irreparableErr.piecesRequired),
				zap.Int("numFailedNodes", len(failedNodeIDs)),
				zap.Stringer("failedNodes", commaSeparatedArray(failedNodeIDs)),
				zap.Int("numOfflineNodes", len(offlineNodeIDs)),
				zap.Stringer("offlineNodes", commaSeparatedArray(offlineNodeIDs)),
				zap.Int("numTimedOutNodes", len(timedOutNodeIDs)),
				zap.Stringer("timedOutNodes", commaSeparatedArray(timedOutNodeIDs)),
				zap.Stringer("unknownErrors", commaSeparatedArray(unknownErrs)),
			)
			// repair will be attempted again if the segment remains unhealthy.
			return false, nil
		}
		// The segment's redundancy strategy is invalid, or else there was an internal error.
		return true, repairReconstructError.New("segment could not be reconstructed: %w", err)
	}
	defer func() { err = errs.Combine(err, segmentReader.Close()) }()

	// only report audit result when segment can be successfully downloaded
	cachedNodesReputation := make(map[storj.NodeID]overlay.ReputationStatus, len(cachedNodesInfo))
	for id, info := range cachedNodesInfo {
		cachedNodesReputation[id] = info.Reputation
	}

	report := audit.Report{
		Segment:         &segment,
		NodesReputation: cachedNodesReputation,
	}

	for _, outcome := range piecesReport.Successful {
		report.Successes = append(report.Successes, outcome.Piece.StorageNode)
	}
	for _, outcome := range piecesReport.Failed {
		report.Fails = append(report.Fails, metabase.Piece{
			StorageNode: outcome.Piece.StorageNode,
			Number:      outcome.Piece.Number,
		})
	}
	for _, outcome := range piecesReport.Offline {
		report.Offlines = append(report.Offlines, outcome.Piece.StorageNode)
	}
	for _, outcome := range piecesReport.Unknown {
		report.Unknown = append(report.Unknown, outcome.Piece.StorageNode)
	}
	if repairer.reputationUpdateEnabled {
		repairer.reporter.RecordAudits(ctx, report)
	}

	// Upload the repaired pieces
	successfulNodes, _, err := repairer.ec.Repair(ctx, putLimits, putPrivateKey, redundancy, segmentReader, repairer.timeout, minSuccessfulNeeded)
	if err != nil {
		return false, repairPutError.Wrap(err)
	}

	pieceSize := eestream.CalcPieceSize(int64(segment.EncryptedSize), redundancy)
	var bytesRepaired int64

	// Add the successfully uploaded pieces to repairedPieces
	var repairedPieces metabase.Pieces
	repairedMap := make(map[uint16]bool)
	for i, node := range successfulNodes {
		if node == nil {
			continue
		}
		bytesRepaired += pieceSize
		piece := metabase.Piece{
			Number:      uint16(i),
			StorageNode: node.Id,
		}
		repairedPieces = append(repairedPieces, piece)
		repairedMap[uint16(i)] = true
	}

	mon.Meter("repair_bytes_uploaded").Mark64(bytesRepaired) //mon:locked

	healthyAfterRepair := numHealthy + len(repairedPieces)
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
		for unhealthyPiece := range unhealthyPieces {
			toRemove = append(toRemove, unhealthyPiece)
		}
	} else {
		// if partial repair, leave unrepaired unhealthy pieces in the pointer
		for unhealthyPiece := range unhealthyPieces {
			if repairedMap[unhealthyPiece.Number] {
				// add only repaired pieces in the slice, unrepaired
				// unhealthy pieces are not removed from the pointer
				toRemove = append(toRemove, unhealthyPiece)
			}
		}
	}

	// add pieces that failed piece hashes verification to the removal list
	for _, outcome := range piecesReport.Failed {
		toRemove = append(toRemove, outcome.Piece)
	}

	newPieces, err := segment.Pieces.Update(repairedPieces, toRemove)
	if err != nil {
		return false, repairPutError.Wrap(err)
	}

	err = repairer.metabase.UpdateSegmentPieces(ctx, metabase.UpdateSegmentPieces{
		StreamID: segment.StreamID,
		Position: segment.Position,

		OldPieces:     segment.Pieces,
		NewRedundancy: segment.Redundancy,
		NewPieces:     newPieces,

		NewRepairedAt: time.Now(),
	})
	if err != nil {
		return false, metainfoPutError.Wrap(err)
	}

	repairedAt := time.Time{}
	if segment.RepairedAt != nil {
		repairedAt = *segment.RepairedAt
	}

	var segmentAge time.Duration
	if segment.CreatedAt.Before(repairedAt) {
		segmentAge = time.Since(repairedAt)
	} else {
		segmentAge = time.Since(segment.CreatedAt)
	}

	// TODO what to do with RepairCount
	var repairCount int64
	// pointer.RepairCount++

	mon.IntVal("segment_time_until_repair").Observe(int64(segmentAge.Seconds())) //mon:locked
	stats.segmentTimeUntilRepair.Observe(int64(segmentAge.Seconds()))
	mon.IntVal("segment_repair_count").Observe(repairCount) //mon:locked
	stats.segmentRepairCount.Observe(repairCount)

	repairer.log.Debug("repaired segment",
		zap.Stringer("Stream ID", segment.StreamID),
		zap.Uint64("Position", segment.Position.Encode()),
		zap.Int("clumped pieces", len(piecesCheck.ClumpedPiecesSet)),
		zap.Int("out of placement pieces", len(piecesCheck.OutOfPlacementPiecesSet)),
		zap.Int("in excluded countries", piecesCheck.NumHealthyInExcludedCountries),
		zap.Int("removed pieces", len(toRemove)),
		zap.Int("repaired pieces", len(repairedPieces)),
		zap.Int("healthy before repair", numHealthy),
		zap.Int("healthy after repair", healthyAfterRepair))
	return true, nil
}

type piecesCheckResult struct {
	ExcludeNodeIDs []storj.NodeID

	MissingPiecesSet map[uint16]bool
	ClumpedPiecesSet map[uint16]bool

	// piece which are out of placement (both offline and online)
	OutOfPlacementPiecesSet map[uint16]bool

	NumUnhealthyRetrievable       int
	NumHealthyInExcludedCountries int
}

func (repairer *SegmentRepairer) classifySegmentPieces(ctx context.Context, segment metabase.Segment) (result piecesCheckResult, err error) {
	defer mon.Task()(&ctx)(&err)

	pieces := segment.Pieces

	allNodeIDs := make([]storj.NodeID, len(pieces))
	for i, piece := range pieces {
		allNodeIDs[i] = piece.StorageNode
	}

	online, offline, err := repairer.overlay.KnownReliable(ctx, allNodeIDs)
	if err != nil {
		return piecesCheckResult{}, overlayQueryError.New("error identifying missing pieces: %w", err)
	}
	return repairer.classifySegmentPiecesWithNodes(ctx, segment, allNodeIDs, online, offline)
}

func (repairer *SegmentRepairer) classifySegmentPiecesWithNodes(ctx context.Context, segment metabase.Segment, allNodeIDs []storj.NodeID, online []nodeselection.SelectedNode, offline []nodeselection.SelectedNode) (result piecesCheckResult, err error) {
	pieces := segment.Pieces

	nodeIDPieceMap := map[storj.NodeID]uint16{}
	result.MissingPiecesSet = map[uint16]bool{}
	for i, p := range pieces {
		allNodeIDs[i] = p.StorageNode
		nodeIDPieceMap[p.StorageNode] = p.Number
		result.MissingPiecesSet[p.Number] = true
	}

	result.ExcludeNodeIDs = allNodeIDs

	nodeFilters := repairer.placementRules(segment.Placement)

	// remove online nodes from missing pieces
	for _, onlineNode := range online {
		// count online nodes in excluded countries only if country is not excluded by segment
		// placement, those nodes will be counted with out of placement check
		if _, excluded := repairer.excludedCountryCodes[onlineNode.CountryCode]; excluded && nodeFilters.MatchInclude(&onlineNode) {
			result.NumHealthyInExcludedCountries++
		}

		pieceNum := nodeIDPieceMap[onlineNode.ID]
		delete(result.MissingPiecesSet, pieceNum)
	}

	if repairer.doDeclumping {
		// if multiple pieces are on the same last_net, keep only the first one. The rest are
		// to be considered retrievable but unhealthy.
		lastNets := make([]string, 0, len(allNodeIDs))

		reliablePieces := metabase.Pieces{}

		collectLastNets := func(reliable []nodeselection.SelectedNode) {
			for _, node := range reliable {
				pieceNum := nodeIDPieceMap[node.ID]
				reliablePieces = append(reliablePieces, metabase.Piece{
					Number:      pieceNum,
					StorageNode: node.ID,
				})
				lastNets = append(lastNets, node.LastNet)
			}
		}
		collectLastNets(online)
		collectLastNets(offline)

		clumpedPieces := repair.FindClumpedPieces(reliablePieces, lastNets)
		result.ClumpedPiecesSet = map[uint16]bool{}
		for _, clumpedPiece := range clumpedPieces {
			result.ClumpedPiecesSet[clumpedPiece.Number] = true
		}
	}

	result.OutOfPlacementPiecesSet = map[uint16]bool{}

	nodeFilters = repairer.placementRules(segment.Placement)
	checkPlacement := func(reliable []nodeselection.SelectedNode) {
		for _, node := range reliable {
			if nodeFilters.MatchInclude(&node) {
				continue
			}

			result.OutOfPlacementPiecesSet[nodeIDPieceMap[node.ID]] = true
		}
	}
	checkPlacement(online)
	checkPlacement(offline)

	// verify that some of clumped pieces and out of placement pieces are not the same
	unhealthyRetrievableSet := map[uint16]bool{}
	maps.Copy(unhealthyRetrievableSet, result.ClumpedPiecesSet)
	maps.Copy(unhealthyRetrievableSet, result.OutOfPlacementPiecesSet)

	// offline nodes are not retrievable
	for _, node := range offline {
		delete(unhealthyRetrievableSet, nodeIDPieceMap[node.ID])
	}
	result.NumUnhealthyRetrievable = len(unhealthyRetrievableSet)

	return result, nil
}

// checkIfSegmentAltered checks if oldSegment has been altered since it was selected for audit.
func (repairer *SegmentRepairer) checkIfSegmentAltered(ctx context.Context, oldSegment metabase.Segment) (err error) {
	defer mon.Task()(&ctx)(&err)

	if repairer.OnTestingCheckSegmentAlteredHook != nil {
		repairer.OnTestingCheckSegmentAlteredHook()
	}

	newSegment, err := repairer.metabase.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
		StreamID: oldSegment.StreamID,
		Position: oldSegment.Position,
	})
	if err != nil {
		if metabase.ErrSegmentNotFound.Has(err) {
			return segmentDeletedError.New("StreamID: %q Position: %d", oldSegment.StreamID.String(), oldSegment.Position.Encode())
		}
		return err
	}

	if !oldSegment.Pieces.Equal(newSegment.Pieces) {
		return segmentModifiedError.New("StreamID: %q Position: %d", oldSegment.StreamID.String(), oldSegment.Position.Encode())
	}
	return nil
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

// SetNow allows tests to have the server act as if the current time is whatever they want.
func (repairer *SegmentRepairer) SetNow(nowFn func() time.Time) {
	repairer.nowFn = nowFn
}

// AdminFetchInfo groups together all the information about a piece that should be retrievable
// from storage nodes.
type AdminFetchInfo struct {
	Reader        io.ReadCloser
	Hash          *pb.PieceHash
	GetLimit      *pb.AddressedOrderLimit
	OriginalLimit *pb.OrderLimit
	FetchError    error
}

// AdminFetchPieces retrieves raw pieces and the associated hashes and original order
// limits from the storage nodes on which they are stored, and returns them intact to
// the caller rather than decoding or decrypting or verifying anything. This is to be
// used for debugging purposes.
func (repairer *SegmentRepairer) AdminFetchPieces(ctx context.Context, seg *metabase.Segment, saveDir string) (pieceInfos []AdminFetchInfo, err error) {
	if seg.Inline() {
		return nil, errs.New("cannot download an inline segment")
	}

	if len(seg.Pieces) < int(seg.Redundancy.RequiredShares) {
		return nil, errs.New("segment only has %d pieces; needs %d for reconstruction", seg.Pieces, seg.Redundancy.RequiredShares)
	}

	// we treat all pieces as "healthy" for our purposes here; we want to download as many
	// of them as we reasonably can. Thus, we pass in seg.Pieces for 'healthy'
	getOrderLimits, getPrivateKey, cachedNodesInfo, err := repairer.orders.CreateGetRepairOrderLimits(ctx, *seg, seg.Pieces)
	if err != nil {
		return nil, errs.New("could not create order limits: %w", err)
	}

	pieceSize := seg.PieceSize()

	pieceInfos = make([]AdminFetchInfo, len(getOrderLimits))
	limiter := sync2.NewLimiter(int(seg.Redundancy.RequiredShares))

	for currentLimitIndex, limit := range getOrderLimits {
		if limit == nil {
			continue
		}
		pieceInfos[currentLimitIndex].GetLimit = limit

		currentLimitIndex, limit := currentLimitIndex, limit
		limiter.Go(ctx, func() {
			info := cachedNodesInfo[limit.GetLimit().StorageNodeId]
			address := limit.GetStorageNodeAddress().GetAddress()
			var triedLastIPPort bool
			if info.LastIPPort != "" && info.LastIPPort != address {
				address = info.LastIPPort
				triedLastIPPort = true
			}

			pieceReadCloser, hash, originalLimit, err := repairer.ec.downloadAndVerifyPiece(ctx, limit, address, getPrivateKey, saveDir, pieceSize)
			// if piecestore dial with last ip:port failed try again with node address
			if triedLastIPPort && piecestore.Error.Has(err) {
				if pieceReadCloser != nil {
					_ = pieceReadCloser.Close()
				}
				pieceReadCloser, hash, originalLimit, err = repairer.ec.downloadAndVerifyPiece(ctx, limit, limit.GetStorageNodeAddress().GetAddress(), getPrivateKey, saveDir, pieceSize)
			}

			pieceInfos[currentLimitIndex].Reader = pieceReadCloser
			pieceInfos[currentLimitIndex].Hash = hash
			pieceInfos[currentLimitIndex].OriginalLimit = originalLimit
			pieceInfos[currentLimitIndex].FetchError = err
		})
	}

	limiter.Wait()

	return pieceInfos, nil
}

// commaSeparatedArray concatenates an array into a comma-separated string,
// lazily.
type commaSeparatedArray []string

func (c commaSeparatedArray) String() string {
	return strings.Join(c, ", ")
}
