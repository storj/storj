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

	// repairThresholdOverrides is the set of values configured by the checker to override the repair threshold for various RS schemes.
	repairThresholdOverrides checker.RepairOverrides
	// repairTargetOverrides is similar but determines the optimum number of pieces per segment.
	repairTargetOverrides checker.RepairOverrides

	excludedCountryCodes map[location.CountryCode]struct{}

	nowFn                            func() time.Time
	OnTestingCheckSegmentAlteredHook func()
	OnTestingPiecesReportHook        func(pieces FetchResultReport)
	placements                       nodeselection.PlacementDefinitions
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
	placements nodeselection.PlacementDefinitions,
	repairThresholdOverrides, repairTargetOverrides checker.RepairOverrides,
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
		repairThresholdOverrides:   repairThresholdOverrides,
		repairTargetOverrides:      repairTargetOverrides,
		excludedCountryCodes:       excludedCountryCodes,
		reporter:                   reporter,
		reputationUpdateEnabled:    config.ReputationUpdateEnabled,
		doDeclumping:               config.DoDeclumping,
		doPlacementCheck:           config.DoPlacementCheck,
		placements:                 placements,

		nowFn: time.Now,
	}
}

// Repair retrieves an at-risk segment and repairs and stores lost pieces on new nodes
// note that shouldDelete is used even in the case where err is not null
// note that it will update audit status as failed for nodes that failed piece hash verification during repair downloading.
func (repairer *SegmentRepairer) Repair(ctx context.Context, queueSegment *queue.InjuredSegment) (shouldDelete bool, err error) {
	defer mon.Task()(&ctx, queueSegment.StreamID.String(), queueSegment.Position.Encode())(&err)

	log := repairer.log.With(zap.Stringer("Stream ID", queueSegment.StreamID), zap.Uint64("Position", queueSegment.Position.Encode()))
	segment, err := repairer.metabase.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
		StreamID: queueSegment.StreamID,
		Position: queueSegment.Position,
	})
	if err != nil {
		if metabase.ErrSegmentNotFound.Has(err) {
			mon.Meter("repair_unnecessary").Mark(1)            //mon:locked
			mon.Meter("segment_deleted_before_repair").Mark(1) //mon:locked
			log.Info("segment was deleted")
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
		log.Info("segment has expired")
		return true, nil
	}

	stats := repairer.getStatsByRS(segment.Redundancy)

	mon.Meter("repair_attempts").Mark(1) //mon:locked
	stats.repairAttempts.Mark(1)
	mon.IntVal("repair_segment_size").Observe(int64(segment.EncryptedSize)) //mon:locked
	stats.repairSegmentSize.Observe(int64(segment.EncryptedSize))

	allNodeIDs := make([]storj.NodeID, len(segment.Pieces))
	for i, p := range segment.Pieces {
		allNodeIDs[i] = p.StorageNode
	}

	selectedNodes, err := repairer.overlay.GetNodes(ctx, allNodeIDs)
	if err != nil {
		return false, overlayQueryError.New("error identifying missing pieces: %w", err)
	}
	if len(selectedNodes) != len(segment.Pieces) {
		log.Error("GetNodes returned an invalid result", zap.Any("pieces", segment.Pieces), zap.Any("selectedNodes", selectedNodes))
		return false, overlayQueryError.New("GetNodes returned an invalid result")
	}
	pieces := segment.Pieces
	piecesCheck := repair.ClassifySegmentPieces(pieces, selectedNodes, repairer.excludedCountryCodes, repairer.doPlacementCheck, repairer.doDeclumping, repairer.placements[segment.Placement])

	newRedundancy := repairer.newRedundancy(segment.Redundancy)

	// irreparable segment
	if piecesCheck.Retrievable.Count() < int(newRedundancy.RequiredShares) {
		mon.Counter("repairer_segments_below_min_req").Inc(1) //mon:locked
		stats.repairerSegmentsBelowMinReq.Inc(1)
		mon.Meter("repair_nodes_unavailable").Mark(1) //mon:locked
		stats.repairerNodesUnavailable.Mark(1)

		log.Warn("irreparable segment",
			zap.Int("piecesAvailable", piecesCheck.Retrievable.Count()),
			zap.Int16("piecesRequired", newRedundancy.RequiredShares),
		)
		return false, nil
	}

	// ensure we get values, even if only zero values, so that redash can have an alert based on this
	mon.Counter("repairer_segments_below_min_req").Inc(0) //mon:locked
	stats.repairerSegmentsBelowMinReq.Inc(0)

	if piecesCheck.Healthy.Count() > int(newRedundancy.RepairShares) {
		// No repair is needed (note Healthy does not include pieces in ForcingRepair).

		var dropPieces metabase.Pieces
		if piecesCheck.ForcingRepair.Count() > 0 {
			// No repair is needed, but remove forcing-repair pieces without a repair operation,
			// as we will still be above the repair threshold.
			for _, piece := range pieces {
				if piecesCheck.ForcingRepair.Contains(int(piece.Number)) {
					dropPieces = append(dropPieces, piece)
				}
			}
			if len(dropPieces) > 0 {
				newPieces, err := segment.Pieces.Update(nil, dropPieces)
				if err != nil {
					return false, metainfoPutError.Wrap(err)
				}

				err = repairer.metabase.UpdateSegmentPieces(ctx, metabase.UpdateSegmentPieces{
					StreamID: segment.StreamID,
					Position: segment.Position,

					OldPieces:     segment.Pieces,
					NewRedundancy: newRedundancy,
					NewPieces:     newPieces,

					NewRepairedAt: time.Now(),
				})
				if err != nil {
					return false, metainfoPutError.Wrap(err)
				}

				mon.Meter("dropped_undesirable_pieces_without_repair").Mark(len(dropPieces))
			}
		}

		mon.Meter("repair_unnecessary").Mark(1) //mon:locked
		stats.repairUnnecessary.Mark(1)
		log.Info("segment above repair threshold",
			zap.Int("numHealthy", piecesCheck.Healthy.Count()),
			zap.Int16("repairThreshold", newRedundancy.RepairShares),
			zap.Int16("repairTarget", newRedundancy.OptimalShares),
			zap.Int("numClumped", piecesCheck.Clumped.Count()),
			zap.Int("numExiting", piecesCheck.Exiting.Count()),
			zap.Int("numOffPieces", piecesCheck.OutOfPlacement.Count()),
			zap.Int("numExcluded", piecesCheck.InExcludedCountry.Count()),
			zap.Int("droppedPieces", len(dropPieces)))
		return true, nil
	}

	healthyRatioBeforeRepair := 0.0
	if segment.Redundancy.TotalShares != 0 {
		healthyRatioBeforeRepair = float64(piecesCheck.Healthy.Count()) / float64(segment.Redundancy.TotalShares)
	}
	mon.FloatVal("healthy_ratio_before_repair").Observe(healthyRatioBeforeRepair) //mon:locked
	stats.healthyRatioBeforeRepair.Observe(healthyRatioBeforeRepair)

	// Create the order limits for the GET_REPAIR action
	retrievablePieces := make(metabase.Pieces, 0, piecesCheck.Retrievable.Count())
	for _, piece := range pieces {
		if piecesCheck.Retrievable.Contains(int(piece.Number)) {
			retrievablePieces = append(retrievablePieces, piece)
		}
	}
	getOrderLimits, getPrivateKey, cachedNodesInfo, err := repairer.orders.CreateGetRepairOrderLimits(ctx, segment, retrievablePieces)
	if err != nil {
		if orders.ErrDownloadFailedNotEnoughPieces.Has(err) {
			mon.Counter("repairer_segments_below_min_req").Inc(1) //mon:locked
			stats.repairerSegmentsBelowMinReq.Inc(1)
			mon.Meter("repair_nodes_unavailable").Mark(1) //mon:locked
			stats.repairerNodesUnavailable.Mark(1)

			log.Warn("irreparable segment: too many nodes offline",
				zap.Int("piecesAvailable", len(retrievablePieces)),
				zap.Int16("piecesRequired", segment.Redundancy.RequiredShares),
				zap.Error(err),
			)
		}
		return false, orderLimitFailureError.New("could not create GET_REPAIR order limits: %w", err)
	}

	// Double check for retrievable pieces which were recognized as irretrievable during the
	// call to CreateGetRepairOrderLimits. Add or remove them from the appropriate sets.
	for _, piece := range retrievablePieces {
		if getOrderLimits[piece.Number] == nil {
			piecesCheck.Missing.Include(int(piece.Number))
			piecesCheck.Unhealthy.Include(int(piece.Number))

			piecesCheck.Healthy.Exclude(int(piece.Number))
			piecesCheck.Retrievable.Exclude(int(piece.Number))
			piecesCheck.UnhealthyRetrievable.Exclude(int(piece.Number))
		}
	}

	var requestCount int
	{
		totalNeeded := int(math.Ceil(float64(newRedundancy.OptimalShares) * repairer.multiplierOptimalThreshold))
		if totalNeeded > int(newRedundancy.TotalShares) {
			totalNeeded = int(newRedundancy.TotalShares)
		}
		requestCount = totalNeeded - piecesCheck.Healthy.Count()
	}
	minSuccessfulNeeded := int(newRedundancy.OptimalShares) - piecesCheck.Healthy.Count()

	var alreadySelected []*nodeselection.SelectedNode
	for i := range selectedNodes {
		alreadySelected = append(alreadySelected, &selectedNodes[i])
	}

	// Request Overlay for n-h new storage nodes
	request := overlay.FindStorageNodesRequest{
		RequestedCount:  requestCount,
		AlreadySelected: alreadySelected,
		Placement:       segment.Placement,
	}

	newNodes, err := repairer.overlay.FindStorageNodesForUpload(ctx, request)
	if err != nil {
		return false, overlayQueryError.Wrap(err)
	}

	oldRedundancyStrategy, err := eestream.NewRedundancyStrategyFromStorj(segment.Redundancy)
	if err != nil {
		return true, invalidRepairError.New("invalid redundancy strategy: %w", err)
	}

	// Download the segment using just the retrievable pieces
	segmentReader, piecesReport, err := repairer.ec.Get(ctx, getOrderLimits, cachedNodesInfo, getPrivateKey, oldRedundancyStrategy, int64(segment.EncryptedSize))

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
			log.Info("segment deleted during Repair")
			return true, nil
		}
		if segmentModifiedError.Has(checkSegmentError) {
			// mon.Meter("segment_modified_during_repair").Mark(1) //mon:locked
			log.Info("segment modified during Repair")
			return true, nil
		}
		return false, segmentVerificationError.Wrap(checkSegmentError)
	}

	if len(piecesReport.Contained) > 0 {
		log.Debug("unexpected contained pieces during repair", zap.Int("count", len(piecesReport.Contained)))
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

			log.Warn("irreparable segment: could not acquire enough shares",
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

	// Create the order limits for the PUT_REPAIR action. We want to keep pieces in Healthy
	// as well as pieces in InExcludedCountry (our policy is to let those nodes keep the
	// pieces they have, as long as they are kept intact and retrievable).
	maxToKeep := int(newRedundancy.TotalShares) - len(newNodes)
	toKeep := map[uint16]struct{}{}

	// TODO how to avoid this two loops
	for _, piece := range pieces {
		if piecesCheck.Healthy.Contains(int(piece.Number)) {
			toKeep[piece.Number] = struct{}{}
		}
	}
	for _, piece := range pieces {
		if piecesCheck.InExcludedCountry.Contains(int(piece.Number)) {
			if len(toKeep) >= maxToKeep {
				break
			}
			toKeep[piece.Number] = struct{}{}
		}
	}

	putLimits, putPrivateKey, err := repairer.orders.CreatePutRepairOrderLimits(ctx, segment, getOrderLimits, toKeep, newNodes)
	if err != nil {
		return false, orderLimitFailureError.New("could not create PUT_REPAIR order limits: %w", err)
	}

	newRedundancyStrategy, err := eestream.NewRedundancyStrategyFromStorj(newRedundancy)
	if err != nil {
		return true, invalidRepairError.New("invalid redundancy strategy: %w", err)
	}

	// Upload the repaired pieces
	successfulNodes, _, err := repairer.ec.Repair(ctx, putLimits, putPrivateKey, newRedundancyStrategy, segmentReader, repairer.timeout, minSuccessfulNeeded)
	if err != nil {
		return false, repairPutError.Wrap(err)
	}

	pieceSize := newRedundancy.PieceSize(int64(segment.EncryptedSize))
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

	healthyAfterRepair := piecesCheck.Healthy.Count() + len(repairedPieces)
	switch {
	case healthyAfterRepair >= int(newRedundancy.OptimalShares):
		mon.Meter("repair_success").Mark(1) //mon:locked
		stats.repairSuccess.Mark(1)
	case healthyAfterRepair <= int(newRedundancy.RepairShares):
		// Important: this indicates a failure to PUT enough pieces to the network to pass
		// the repair threshold, and _not_ a failure to reconstruct the segment. But we
		// put at least one piece, else ec.Repair() would have returned an error. So the
		// repair "succeeded" in that the segment is now healthier than it was, but it is
		// not as healthy as we want it to be.
		mon.Meter("repair_failed").Mark(1) //mon:locked
		stats.repairFailed.Mark(1)
	default:
		mon.Meter("repair_partial").Mark(1) //mon:locked
		stats.repairPartial.Mark(1)
	}

	healthyRatioAfterRepair := 0.0
	if newRedundancy.TotalShares != 0 {
		healthyRatioAfterRepair = float64(healthyAfterRepair) / float64(newRedundancy.TotalShares)
	}

	mon.FloatVal("healthy_ratio_after_repair").Observe(healthyRatioAfterRepair) //mon:locked
	stats.healthyRatioAfterRepair.Observe(healthyRatioAfterRepair)

	toRemove := make(map[uint16]metabase.Piece, piecesCheck.Unhealthy.Count())
	switch {
	case healthyAfterRepair >= int(newRedundancy.OptimalShares):
		// Repair was fully successful; remove all unhealthy pieces except those in
		// (Retrievable AND InExcludedCountry). Those, we allow to remain on the nodes as
		// long as the nodes are keeping the pieces intact and available.
		for _, piece := range pieces {
			if piecesCheck.Unhealthy.Contains(int(piece.Number)) {
				retrievable := piecesCheck.Retrievable.Contains(int(piece.Number))
				inExcludedCountry := piecesCheck.InExcludedCountry.Contains(int(piece.Number))
				if retrievable && inExcludedCountry {
					continue
				}
				toRemove[piece.Number] = piece
			}
		}
	case healthyAfterRepair > int(newRedundancy.RepairShares):
		// Repair was successful enough that we still want to drop all out-of-placement
		// pieces. We want to do that wherever possible, except where doing so puts data in
		// jeopardy.
		for _, piece := range pieces {
			if piecesCheck.OutOfPlacement.Contains(int(piece.Number)) {
				toRemove[piece.Number] = piece
			}
		}
	default:
		// Repair improved the health of the piece, but it is still at or below the
		// repair threshold (not counting unhealthy-but-retrievable pieces). To be safe,
		// we will keep unhealthy-but-retrievable pieces in the segment for now.
	}

	// in any case, we want to remove pieces for which we have replacements now.
	for _, piece := range pieces {
		if repairedMap[piece.Number] {
			toRemove[piece.Number] = piece
		}
	}

	// add pieces that failed piece hash verification to the removal list
	for _, outcome := range piecesReport.Failed {
		toRemove[outcome.Piece.Number] = outcome.Piece
	}

	newPieces, err := segment.Pieces.Update(repairedPieces, maps.Values(toRemove))
	if err != nil {
		return false, repairPutError.Wrap(err)
	}

	err = repairer.metabase.UpdateSegmentPieces(ctx, metabase.UpdateSegmentPieces{
		StreamID: segment.StreamID,
		Position: segment.Position,

		OldPieces:     segment.Pieces,
		NewRedundancy: newRedundancy,
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

	mon.IntVal("segment_time_until_repair").Observe(int64(segmentAge.Seconds())) //mon:locked
	stats.segmentTimeUntilRepair.Observe(int64(segmentAge.Seconds()))

	log.Info("repaired segment",
		zap.Int("clumped pieces", piecesCheck.Clumped.Count()),
		zap.Int("exiting-node pieces", piecesCheck.Exiting.Count()),
		zap.Int("out of placement pieces", piecesCheck.OutOfPlacement.Count()),
		zap.Int("in excluded countries", piecesCheck.InExcludedCountry.Count()),
		zap.Int("missing pieces", piecesCheck.Missing.Count()),
		zap.Int("removed pieces", len(toRemove)),
		zap.Int("repaired pieces", len(repairedPieces)),
		zap.Int("retrievable pieces", piecesCheck.Retrievable.Count()),
		zap.Int("healthy before repair", piecesCheck.Healthy.Count()),
		zap.Int("healthy after repair", healthyAfterRepair),
		zap.Int("total before repair", len(selectedNodes)),
		zap.Int("total after repair", len(newPieces)))
	return true, nil
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

func (repairer *SegmentRepairer) getStatsByRS(redundancy storj.RedundancyScheme) *stats {
	return repairer.statsCollector.getStatsByRS(getRSString(redundancy))
}

func (repairer *SegmentRepairer) newRedundancy(redundancy storj.RedundancyScheme) storj.RedundancyScheme {
	if overrideValue := repairer.repairThresholdOverrides.GetOverrideValue(redundancy); overrideValue != 0 {
		redundancy.RepairShares = int16(overrideValue)
	}
	if overrideValue := repairer.repairTargetOverrides.GetOverrideValue(redundancy); overrideValue != 0 {
		redundancy.OptimalShares = int16(overrideValue)
	}
	return redundancy
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
