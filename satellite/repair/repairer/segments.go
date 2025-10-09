// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package repairer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"time"

	"github.com/calebcase/tmpfile"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/exp/maps"

	"storj.io/common/errs2"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/eventkit"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/repair"
	"storj.io/storj/satellite/repair/checker"
	"storj.io/storj/satellite/repair/queue"
	"storj.io/storj/shared/location"
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
	// segmentVerificationError is the errs class when the repaired segment can not be verified during repair.
	segmentVerificationError = errs.Class("segment verification failed")
	// segmentDeletedError is the errs class when the repaired segment was deleted during the repair.
	segmentDeletedError = errs.Class("segment deleted during repair")
	// segmentModifiedError is the errs class used when a segment has been changed in any way.
	segmentModifiedError = errs.Class("segment has been modified")

	ek = eventkit.Package()
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

// participatingNodesCache alias for making the code a bit nicer below.
// This chaches the nodes retrieved from the database to upload repaired pieces.
type participatingNodesCache = sync2.ReadCacheOf[map[storj.NodeID]*nodeselection.SelectedNode]

// nodesForRepairCache alias for making the code a bit nicer below.
// This caches the nodes retrieved from the database for getting repair orders.
type nodesForRepairCache = sync2.ReadCacheOf[map[storj.NodeID]*overlay.NodeReputation]

// SegmentRepairer for segments.
type SegmentRepairer struct {
	log            *zap.Logger
	statsCollector *statsCollector
	metabase       *metabase.DB
	orders         *orders.Service
	overlay        Overlay
	ec             *ECRepairer
	timeout        time.Duration
	reporter       audit.Reporter

	participatingNodesCache *participatingNodesCache
	nodesForRepairCache     *nodesForRepairCache

	reputationUpdateEnabled bool
	doDeclumping            bool
	doPlacementCheck        bool

	// multiplierOptimalThreshold is the value that multiplied by the optimal
	// threshold results in the maximum limit of number of nodes to upload
	// repaired pieces
	multiplierOptimalThreshold float64

	// repairThresholdOverrides is the set of values configured by the checker to override the repair threshold for various RS schemes.
	repairThresholdOverrides checker.RepairThresholdOverrides
	// repairTargetOverrides is similar but determines the optimum number of pieces per segment.
	repairTargetOverrides checker.RepairTargetOverrides

	excludedCountryCodes map[location.CountryCode]struct{}

	nowFn                            func() time.Time
	OnTestingCheckSegmentAlteredHook func()
	OnTestingPiecesReportHook        func(pieces FetchResultReport)
	placements                       nodeselection.PlacementDefinitions
	// onlineWindow to consider if storage nodes are online according to their last successful contact.
	onlineWindow time.Duration
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
	overlaysvc *overlay.Service,
	reporter audit.Reporter,
	ecRepairer *ECRepairer,
	placements nodeselection.PlacementDefinitions,
	repairThresholdOverrides checker.RepairThresholdOverrides,
	repairTargetOverrides checker.RepairTargetOverrides,
	config Config,
) (_ *SegmentRepairer, err error) {

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

	repairer := &SegmentRepairer{
		log:                        log,
		statsCollector:             newStatsCollector(),
		metabase:                   metabase,
		orders:                     orders,
		overlay:                    overlaysvc,
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
		onlineWindow:               config.OnlineWindow,

		nowFn: time.Now,
	}

	if config.ParticipatingNodeCacheEnabled {
		repairer.participatingNodesCache, err = sync2.NewReadCache(config.ParticipatingNodeCacheInterval, config.ParticipatingNodeCacheStale, repairer.fetchParticipatingNodes)
		if err != nil {
			return nil, Error.Wrap(err)
		}
	}

	if config.NodesForRepairCacheEnabled {
		repairer.nodesForRepairCache, err = sync2.NewReadCache(
			config.NodesForRepairCacheInterval, config.NodesForRepairCacheStale, func(ctx context.Context) (map[storj.NodeID]*overlay.NodeReputation, error) {
				return overlaysvc.GetAllOnlineNodesForRepair(ctx, repairer.onlineWindow)
			})
		if err != nil {
			return nil, Error.Wrap(err)
		}
	}

	return repairer, nil
}

// Run background services needed for segment repair.
func (repairer *SegmentRepairer) Run(ctx context.Context) error {
	if repairer.participatingNodesCache == nil && repairer.nodesForRepairCache == nil {
		return nil
	}

	group := errs2.Group{}

	if repairer.participatingNodesCache != nil {
		group.Go(func() error {
			return repairer.participatingNodesCache.Run(ctx)

		})
	}

	if repairer.nodesForRepairCache != nil {
		group.Go(func() error {
			return repairer.nodesForRepairCache.Run(ctx)
		})
	}

	return errs.Combine(group.Wait()...)
}

// Repair retrieves an at-risk segment and repairs and stores lost pieces on new nodes
// note that shouldDelete is used even in the case where err is not null
// note that it will update audit status as failed for nodes that failed piece hash verification during repair downloading.
func (repairer *SegmentRepairer) Repair(ctx context.Context, queueSegment queue.InjuredSegment) (shouldDelete bool, err error) {
	defer mon.Task()(&ctx, queueSegment.StreamID.String(), queueSegment.Position.Encode(), queueSegment.Placement)(&err)

	log := repairer.log.With(
		zap.Stringer("Stream ID", queueSegment.StreamID),
		zap.Uint64("Position", queueSegment.Position.Encode()),
		zap.Uint16("Placement", uint16(queueSegment.Placement)),
	)

	segment, err := repairer.metabase.GetSegmentByPositionForRepair(ctx, metabase.GetSegmentByPosition{
		StreamID: queueSegment.StreamID,
		Position: queueSegment.Position,
	})
	if err != nil {
		if metabase.ErrSegmentNotFound.Has(err) {
			p := monkit.SeriesTag{Key: "placement", Val: strconv.FormatUint(uint64(queueSegment.Placement), 10)}
			mon.Meter("repair_unnecessary", p).Mark(1)
			mon.Meter("segment_deleted_before_repair", p).Mark(1)
			log.Info("segment was deleted")
			return true, nil
		}
		return false, metainfoGetError.Wrap(err)
	}

	if segment.Inline() {
		return true, invalidRepairError.New("cannot repair inline segment")
	}

	stats := repairer.getStats(segment.Redundancy, segment.Placement)
	// ignore segment if expired
	if segment.Expired(repairer.nowFn()) {
		stats.repairUnnecessary.Mark(1)
		stats.segmentExpiredBeforeRepair.Mark(1)
		log.Info("segment has expired")
		return true, nil
	}

	stats.repairAttempts.Mark(1)
	stats.repairSegmentSize.Observe(int64(segment.EncryptedSize))

	allNodeIDs := make([]storj.NodeID, len(segment.Pieces))
	for i, p := range segment.Pieces {
		allNodeIDs[i] = p.StorageNode
	}

	selectedNodes, err := repairer.getParticipatingNodes(ctx, allNodeIDs)
	if err != nil {
		return false, overlayQueryError.New("error identifying missing pieces: %w", err)
	}
	if len(selectedNodes) != len(segment.Pieces) {
		log.Error("GetParticipatingNodes returned an invalid result", zap.Any("pieces", segment.Pieces), zap.Any("selectedNodes", selectedNodes))
		return false, overlayQueryError.New("GetParticipatingNodes returned an invalid result")
	}
	pieces := segment.Pieces
	placementDef := repairer.placements[segment.Placement]
	piecesCheck := repair.ClassifySegmentPieces(pieces, selectedNodes, repairer.excludedCountryCodes, repairer.doPlacementCheck, repairer.doDeclumping, placementDef)

	newRedundancy := checker.AdjustRedundancy(segment.Redundancy, repairer.repairThresholdOverrides, repairer.repairTargetOverrides, repairer.placements[segment.Placement])

	// irreparable segment
	if piecesCheck.Retrievable.Count() < int(newRedundancy.RequiredShares) {
		stats.repairerSegmentsBelowMinReq.Inc(1)
		stats.repairerNodesUnavailable.Mark(1) //mon::locked

		log.Warn("irreparable segment",
			zap.Int("Pieces Available", piecesCheck.Retrievable.Count()),
			zap.Int16("Pieces Required", newRedundancy.RequiredShares),
		)
		tags := make([]eventkit.Tag, 0, 18)
		tags = append(tags,
			eventkit.Bytes("stream-id", queueSegment.StreamID.Bytes()),
			eventkit.Int64("stream-position", int64(queueSegment.Position.Encode())),
			eventkit.Int64("segment-size", int64(segment.EncryptedSize)),
			eventkit.Int64("placement", int64(segment.Placement)),
			eventkit.Int64("pieces-required", int64(newRedundancy.RequiredShares)),
			eventkit.Int64("pieces-missing", int64(piecesCheck.Missing.Count())),
			eventkit.Int64("pieces-retrievable", int64(piecesCheck.Retrievable.Count())),
			eventkit.Int64("pieces-suspended", int64(piecesCheck.Suspended.Count())),
			eventkit.Int64("pieces-clumped", int64(piecesCheck.Clumped.Count())),
			eventkit.Int64("pieces-exiting", int64(piecesCheck.Exiting.Count())),
			eventkit.Int64("pieces-out-of-placement", int64(piecesCheck.OutOfPlacement.Count())),
			eventkit.Int64("pieces-in-excluded-country", int64(piecesCheck.InExcludedCountry.Count())),
			eventkit.Int64("pieces-forcing-repair", int64(piecesCheck.ForcingRepair.Count())),
			eventkit.Int64("pieces-unhealthy", int64(piecesCheck.Unhealthy.Count())),
			eventkit.Int64("pieces-healthy", int64(piecesCheck.Healthy.Count())),
			eventkit.Timestamp("created-at", segment.CreatedAt),
		)
		if segment.RepairedAt != nil {
			tags = append(tags, eventkit.Timestamp("repaired-at", *segment.RepairedAt))
		}
		if segment.ExpiresAt != nil {
			tags = append(tags, eventkit.Timestamp("expires-at", *segment.ExpiresAt))
		}
		ek.Event("irreparable_segment", tags...)

		return false, nil
	}

	// ensure we get values, even if only zero values, so that redash can have an alert based on this
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

				stats.droppedUndesirablePiecesWithoutRepiar.Mark(len(dropPieces))
			}
		}

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
	stats.healthyRatioBeforeRepair.Observe(healthyRatioBeforeRepair)

	// Create the order limits for the GET_REPAIR action
	retrievablePieces := make(metabase.Pieces, 0, piecesCheck.Retrievable.Count())
	for _, piece := range pieces {
		if piecesCheck.Retrievable.Contains(int(piece.Number)) {
			retrievablePieces = append(retrievablePieces, piece)
		}
	}
	getOrderLimits, getPrivateKey, cachedNodesInfo, err := repairer.orders.CreateGetRepairOrderLimits(
		ctx, segment, retrievablePieces, repairer.getNodesForRepair,
	)
	if err != nil {
		if orders.ErrDownloadFailedNotEnoughPieces.Has(err) {
			stats.repairerSegmentsBelowMinReq.Inc(1)
			mon.Meter("repair_nodes_unavailable").Mark(1)
			stats.repairerNodesUnavailable.Mark(1)

			log.Warn("irreparable segment: too many nodes offline",
				zap.Int("Pieces Available", len(retrievablePieces)),
				zap.Int16("Pieces Required", segment.Redundancy.RequiredShares),
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

	// we should download at least segment.Redundancy.RequiredShares, but sometimes it's enough to download unhealthy but retrievable pieces
	// here we estimate the benefit of using a direct download approach
	if requestCount <= piecesCheck.UnhealthyRetrievable.Count() && // we have enough unhealthy-retrievable, to use them without segment recreation
		requestCount < int(segment.Redundancy.RequiredShares) { // it's better to download unhealthy-retrievable, as it causes fewer downloads
		// we can use unhealthy retrievable pieces, instead of reconstruct segments.

		// instead of required_shares, we would download only the requestCount
		stats.repairerUnnecessaryDownloads.Inc(int64(segment.Redundancy.RequiredShares) - int64(requestCount))
	}
	stats.repairerRequiredDownloads.Inc(int64(requestCount))

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

	log.Debug("fetching pieces for segment",
		zap.Int("numOrderLimits", len(getOrderLimits)),
		zap.Stringer("RS", segment.Redundancy))

	// this will pass additional info to restored from trash event sent by underlying libuplink
	//nolint: revive
	//lint:ignore SA1029 this is a temporary solution
	ctx = context.WithValue(ctx, "restored_from_trash", map[string]string{
		"StreamID":       queueSegment.StreamID.String(),
		"StreamPosition": strconv.Itoa(int(queueSegment.Position.Encode())),
	})
	// Download the segment using just the retrievable pieces
	segmentReader, piecesReport, err := repairer.ec.Get(ctx, log, getOrderLimits, cachedNodesInfo, getPrivateKey, oldRedundancyStrategy, int64(segment.EncryptedSize))

	// ensure we get values, even if only zero values, so that redash can have an alert based on this
	stats.repairTooManyNodesFailed.Mark(0)
	stats.repairSuspectedNetworkProblem.Mark(0)

	if repairer.OnTestingPiecesReportHook != nil {
		repairer.OnTestingPiecesReportHook(piecesReport)
	}

	// Check if segment has been altered
	checkSegmentError := repairer.checkIfSegmentAltered(ctx, segment)
	if checkSegmentError != nil {
		if segmentDeletedError.Has(checkSegmentError) {
			log.Info("segment deleted during Repair")
			return true, nil
		}
		if segmentModifiedError.Has(checkSegmentError) {
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
				stats.repairSuspectedNetworkProblem.Mark(1)
			} else {
				stats.repairTooManyNodesFailed.Mark(1)
			}
			stats.repairTooManyNodesFailed.Mark(1)

			failedNodeIDs := make([]storj.NodeID, 0, len(piecesReport.Failed))
			offlineNodeIDs := make([]storj.NodeID, 0, len(piecesReport.Offline))
			timedOutNodeIDs := make([]storj.NodeID, 0, len(piecesReport.Contained))
			unknownErrs := make([]string, 0, len(piecesReport.Unknown))
			for _, outcome := range piecesReport.Failed {
				failedNodeIDs = append(failedNodeIDs, outcome.Piece.StorageNode)
			}
			for _, outcome := range piecesReport.Offline {
				offlineNodeIDs = append(offlineNodeIDs, outcome.Piece.StorageNode)
			}
			for _, outcome := range piecesReport.Contained {
				timedOutNodeIDs = append(timedOutNodeIDs, outcome.Piece.StorageNode)
			}
			for _, outcome := range piecesReport.Unknown {
				// We are purposefully using the error's string here, as opposed
				// to wrapping the error. It is not likely that we need the local-side
				// traceback of where this error was initially wrapped, and this will
				// keep the logs more readable.
				unknownErrs = append(unknownErrs, fmt.Sprintf("node ID [%s] err: %v", outcome.Piece.StorageNode, outcome.Err))
			}

			log.Warn("irreparable segment: could not acquire enough shares",
				zap.Int32("Pieces Available", irreparableErr.piecesAvailable),
				zap.Int32("Pieces Required", irreparableErr.piecesRequired),
				zap.Int("Failed Nodes", len(failedNodeIDs)),
				zap.Stringers("Failed Nodes List", failedNodeIDs),
				zap.Int("Offline Nodes", len(offlineNodeIDs)),
				zap.Stringers("Offline Nodes List", offlineNodeIDs),
				zap.Int("Timed Out Nodes", len(timedOutNodeIDs)),
				zap.Stringers("Timed Out Nodes List", timedOutNodeIDs),
				zap.Strings("Unknown Errors List", unknownErrs),
				zap.Uint16("Placement", uint16(segment.Placement)),
			)

			tags := make([]eventkit.Tag, 0, 23)
			tags = append(tags,
				eventkit.Bytes("stream-id", queueSegment.StreamID.Bytes()),
				eventkit.Int64("stream-position", int64(queueSegment.Position.Encode())),
				eventkit.Int64("segment-size", int64(segment.EncryptedSize)),
				eventkit.Int64("placement", int64(segment.Placement)),
				eventkit.Int64("pieces-required", int64(newRedundancy.RequiredShares)),
				eventkit.Int64("pieces-missing", int64(piecesCheck.Missing.Count())),
				eventkit.Int64("pieces-retrievable", int64(piecesCheck.Retrievable.Count())),
				eventkit.Int64("pieces-suspended", int64(piecesCheck.Suspended.Count())),
				eventkit.Int64("pieces-clumped", int64(piecesCheck.Clumped.Count())),
				eventkit.Int64("pieces-exiting", int64(piecesCheck.Exiting.Count())),
				eventkit.Int64("pieces-out-of-placement", int64(piecesCheck.OutOfPlacement.Count())),
				eventkit.Int64("pieces-in-excluded-country", int64(piecesCheck.InExcludedCountry.Count())),
				eventkit.Int64("pieces-forcing-repair", int64(piecesCheck.ForcingRepair.Count())),
				eventkit.Int64("pieces-unhealthy", int64(piecesCheck.Unhealthy.Count())),
				eventkit.Int64("pieces-healthy", int64(piecesCheck.Healthy.Count())),
				eventkit.Timestamp("created-at", segment.CreatedAt),
				eventkit.Int64("piece-fetch-successful", int64(len(piecesReport.Successful))),
				eventkit.Int64("piece-fetch-failed", int64(len(piecesReport.Failed))),
				eventkit.Int64("piece-fetch-offline", int64(len(piecesReport.Offline))),
				eventkit.Int64("piece-fetch-contained", int64(len(piecesReport.Contained))),
				eventkit.Int64("piece-fetch-unknown", int64(len(piecesReport.Unknown))),
			)
			if segment.RepairedAt != nil {
				tags = append(tags, eventkit.Timestamp("repaired-at", *segment.RepairedAt))
			}
			if segment.ExpiresAt != nil {
				tags = append(tags, eventkit.Timestamp("expires-at", *segment.ExpiresAt))
			}
			ek.Event("irretrievable_segment", tags...)

			// repair will be attempted again if the segment remains unhealthy.
			return false, nil
		}
		// The segment's redundancy strategy is invalid, or else there was an internal error.
		return false, repairReconstructError.New("segment could not be reconstructed: %w", err)
	}
	defer func() { err = errs.Combine(err, segmentReader.Close()) }()

	// Reconstruct the segment from the pieces. This should ideally happen in
	// tandem with the new piece uploads (have Repair() read directly from
	// segmentReader), but this causes a situation where slow uploads cause
	// reads from the piece files to block due to backpressure, but then the
	// slow reads are marked as inactive (a "internal: quiescence" error is
	// returned). This causes all uploads to fail. Instead, for now, we will
	// write the reconstructed segment to a tempfile and then upload the pieces
	// from that.
	//
	// Once it is possible to suppress or avoid the quiescence error in
	// eestream.decodedReader, we can remove this tempfile step.
	if !repairer.ec.inmemoryDownload {
		err := func() (err error) {
			tempfile, err := tmpfile.New("", "repaired-segment-*")
			if err != nil {
				return repairReconstructError.New("could not open tempfile: %w", err)
			}
			defer func() {
				if recoverErr := recover(); recoverErr != nil {
					err = repairReconstructError.New("panic during segment reconstruction: %v", recoverErr)
				}
				if err != nil {
					_ = tempfile.Close()
				}
			}()
			_, err = io.Copy(tempfile, segmentReader)
			if err != nil {
				return repairReconstructError.New("could not reconstruct segment: %w", err)
			}
			_, err = tempfile.Seek(0, io.SeekStart)
			if err != nil {
				return repairReconstructError.New("could not seek to beginning of tempfile: %w", err)
			}
			err = segmentReader.Close()
			if err != nil {
				return repairReconstructError.New("could not close segmentReader: %w", err)
			}
			// assign tempfile before proceeding, because we've already defer-closed segmentReader
			segmentReader = tempfile
			return nil
		}()
		if err != nil {
			return false, err
		}
	}

	// only report audit result when segment can be successfully downloaded
	cachedNodesReputation := make(map[storj.NodeID]overlay.ReputationStatus, len(cachedNodesInfo))
	for id, info := range cachedNodesInfo {
		cachedNodesReputation[id] = info.Reputation
	}

	segmentAudit := metabase.SegmentForAudit(segment)
	report := audit.Report{
		Segment:         &segmentAudit,
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

	putLimits, putPrivateKey, err := repairer.orders.CreatePutRepairOrderLimits(ctx, segment, newRedundancy, getOrderLimits, toKeep, newNodes)
	if err != nil {
		return false, orderLimitFailureError.New("could not create PUT_REPAIR order limits: %w", err)
	}

	newRedundancyStrategy, err := eestream.NewRedundancyStrategyFromStorj(newRedundancy)
	if err != nil {
		return true, invalidRepairError.New("invalid redundancy strategy: %w", err)
	}

	log.Debug("putting pieces for segment",
		zap.Int("numOrderLimits", len(putLimits)),
		zap.Int("minSuccessfulNeeded", minSuccessfulNeeded),
		zap.Stringer("RS", newRedundancy))

	// Upload the repaired pieces
	successfulNodes, _, err := repairer.ec.Repair(ctx, log, putLimits, putPrivateKey, newRedundancyStrategy, segmentReader, repairer.timeout, minSuccessfulNeeded)
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

	stats.repairBytesUploaded.Mark64(bytesRepaired)

	healthyAfterRepair := piecesCheck.Healthy.Count() + len(repairedPieces)
	switch {
	case healthyAfterRepair >= int(newRedundancy.OptimalShares):
		stats.repairSuccess.Mark(1)
	case healthyAfterRepair <= int(newRedundancy.RepairShares):
		// Important: this indicates a failure to PUT enough pieces to the network to pass
		// the repair threshold, and _not_ a failure to reconstruct the segment. But we
		// put at least one piece, else ec.Repair() would have returned an error. So the
		// repair "succeeded" in that the segment is now healthier than it was, but it is
		// not as healthy as we want it to be.
		stats.repairFailed.Mark(1)
	default:
		stats.repairPartial.Mark(1)
	}

	healthyRatioAfterRepair := 0.0
	if newRedundancy.TotalShares != 0 {
		healthyRatioAfterRepair = float64(healthyAfterRepair) / float64(newRedundancy.TotalShares)
	}

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
func (repairer *SegmentRepairer) checkIfSegmentAltered(ctx context.Context, oldSegment metabase.SegmentForRepair) (err error) {
	defer mon.Task()(&ctx)(&err)

	if repairer.OnTestingCheckSegmentAlteredHook != nil {
		repairer.OnTestingCheckSegmentAlteredHook()
	}

	altered, err := repairer.metabase.CheckSegmentPiecesAlteration(ctx, oldSegment.StreamID, oldSegment.Position, oldSegment.Pieces)
	if err != nil {
		if metabase.ErrSegmentNotFound.Has(err) {
			return segmentDeletedError.New("StreamID: %q Position: %d", oldSegment.StreamID.String(), oldSegment.Position.Encode())
		}

		return err
	}

	if altered {
		return segmentModifiedError.New("StreamID: %q Position: %d", oldSegment.StreamID.String(), oldSegment.Position.Encode())
	}

	return nil
}

func (repairer *SegmentRepairer) getStats(r storj.RedundancyScheme, p storj.PlacementConstraint) *stats {
	return repairer.statsCollector.getStats(getRSString(r), getPlacementString(p))
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
func (repairer *SegmentRepairer) AdminFetchPieces(
	ctx context.Context, log *zap.Logger, seg *metabase.SegmentForRepair, saveDir string,
) (pieceInfos []AdminFetchInfo, err error) {
	if seg.Inline() {
		return nil, errs.New("cannot download an inline segment")
	}

	if len(seg.Pieces) < int(seg.Redundancy.RequiredShares) {
		return nil, errs.New("segment only has %d pieces; needs %d for reconstruction", seg.Pieces, seg.Redundancy.RequiredShares)
	}

	// we treat all pieces as "healthy" for our purposes here; we want to download as many
	// of them as we reasonably can. Thus, we pass in seg.Pieces for 'healthy'
	getOrderLimits, getPrivateKey, cachedNodesInfo, err := repairer.orders.CreateGetRepairOrderLimits(
		ctx, *seg, seg.Pieces, repairer.getNodesForRepair,
	)
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

			log.Debug("piece download attempt",
				zap.Stringer("Node ID", limit.Limit.StorageNodeId),
				zap.Stringer("Piece ID", limit.Limit.PieceId),
				zap.Int("piece index", currentLimitIndex),
				zap.String("address", limit.GetStorageNodeAddress().Address),
				zap.String("last_ip_port", info.LastIPPort),
				zap.Binary("serial", limit.Limit.SerialNumber[:]))

			var triedLastIPPort bool
			if info.LastIPPort != "" && info.LastIPPort != address {
				address = info.LastIPPort
				triedLastIPPort = true
			}

			pieceReadCloser, hash, originalLimit, err := repairer.ec.downloadAndVerifyPiece(ctx, limit, address, getPrivateKey, saveDir, pieceSize)
			// if piecestore dial with last ip:port failed try again with node address
			if triedLastIPPort && ErrDialFailed.Has(err) {
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

func (repairer *SegmentRepairer) getParticipatingNodes(ctx context.Context, nodes []storj.NodeID) ([]nodeselection.SelectedNode, error) {
	if repairer.participatingNodesCache == nil {
		return repairer.overlay.GetParticipatingNodesForRepair(ctx, nodes, repairer.onlineWindow)
	}

	cache, err := repairer.participatingNodesCache.Get(ctx, time.Now())
	if err != nil {
		return nil, Error.Wrap(err)
	}

	result := make([]nodeselection.SelectedNode, len(nodes))
	for i, id := range nodes {
		node, ok := cache[id]
		if !ok {
			continue
		}
		result[i] = *node
	}

	return result, nil
}

func (repairer *SegmentRepairer) fetchParticipatingNodes(ctx context.Context) (map[storj.NodeID]*nodeselection.SelectedNode, error) {
	nodes, err := repairer.overlay.GetAllParticipatingNodesForRepair(ctx, repairer.onlineWindow)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	selected := make(map[storj.NodeID]*nodeselection.SelectedNode, len(nodes))
	for i := range nodes {
		selected[nodes[i].ID] = &nodes[i]
	}

	return selected, nil
}

// RefreshParticipatingNodesCache refreshes internal participating nodes cache.
func (repairer *SegmentRepairer) RefreshParticipatingNodesCache(ctx context.Context) error {
	if repairer.participatingNodesCache == nil {
		return nil
	}
	_, err := repairer.participatingNodesCache.RefreshAndGet(ctx, time.Now())
	return err
}

func (repairer *SegmentRepairer) getNodesForRepair(ctx context.Context, nodes []storj.NodeID) (map[storj.NodeID]*overlay.NodeReputation, error) {
	if repairer.nodesForRepairCache == nil {
		return repairer.overlay.GetOnlineNodesForRepair(ctx, nodes, repairer.onlineWindow)
	}

	// We return all. We could filter out and return the asked ones, but it doesn't hurt to return
	// not asked nodes and save the computation to filter them out.
	result, err := repairer.nodesForRepairCache.Get(ctx, time.Now())
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return result, nil
}
