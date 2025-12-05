// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"strconv"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/errs2"
	"storj.io/common/identity"
	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/rpc"
	"storj.io/common/rpc/rpcpool"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/storj"
	"storj.io/eventkit"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/uplink/private/eestream"
	"storj.io/uplink/private/piecestore"
)

var (
	mon = monkit.Package()
	evs = eventkit.Package()

	// ErrNotEnoughShares is the errs class for when not enough shares are available to do an audit.
	ErrNotEnoughShares = errs.Class("not enough shares for successful audit")
	// ErrSegmentDeleted is the errs class when the audited segment was deleted during the audit.
	ErrSegmentDeleted = errs.Class("segment deleted during audit")
	// ErrSegmentModified is the errs class used when a segment has been changed in any way.
	ErrSegmentModified = errs.Class("segment has been modified")
)

// FailurePhase indicates during which phase a GET_SHARE operation failed.
type FailurePhase int

const (
	// NoFailure indicates there was no failure during a GET_SHARE operation.
	NoFailure FailurePhase = 0
	// DialFailure indicates a GET_SHARE operation failed during Dial.
	DialFailure FailurePhase = 1
	// RequestFailure indicates a GET_SHARE operation failed to make its RPC request, or the request failed.
	RequestFailure FailurePhase = 2
)

// Share represents required information about an audited share.
type Share struct {
	Error        error
	FailurePhase FailurePhase
	PieceNum     int
	NodeID       storj.NodeID
	Data         []byte
}

// Verifier helps verify the correctness of a given stripe.
//
// architecture: Worker
type Verifier struct {
	log                *zap.Logger
	metabase           *metabase.DB
	orders             *orders.Service
	auditor            *identity.PeerIdentity
	dialer             rpc.Dialer
	overlay            *overlay.Service
	containment        Containment
	minBytesPerSecond  memory.Size
	minDownloadTimeout time.Duration

	nowFn                            func() time.Time
	OnTestingCheckSegmentAlteredHook func()
}

// NewVerifier creates a Verifier.
func NewVerifier(log *zap.Logger, metabase *metabase.DB, dialer rpc.Dialer, overlay *overlay.Service, containment Containment, orders *orders.Service, id *identity.FullIdentity, minBytesPerSecond memory.Size, minDownloadTimeout time.Duration) *Verifier {
	return &Verifier{
		log:                log,
		metabase:           metabase,
		orders:             orders,
		auditor:            id.PeerIdentity(),
		dialer:             dialer,
		overlay:            overlay,
		containment:        containment,
		minBytesPerSecond:  minBytesPerSecond,
		minDownloadTimeout: minDownloadTimeout,
		nowFn:              time.Now,
	}
}

// Verify downloads shares then verifies the data correctness at a random stripe.
func (verifier *Verifier) Verify(ctx context.Context, segment Segment, skip map[storj.NodeID]bool) (report Report, err error) {
	defer mon.Task()(&ctx)(&err)

	var segmentInfo metabase.SegmentForAudit
	defer func() {
		recordStats(report, len(segmentInfo.Pieces), err)
	}()

	segmentInfo, err = verifier.metabase.GetSegmentByPositionForAudit(ctx, metabase.GetSegmentByPosition{
		StreamID: segment.StreamID,
		Position: segment.Position,
	})
	if err != nil {
		if metabase.ErrSegmentNotFound.Has(err) {
			verifier.log.Debug("segment deleted before Verify")
			return Report{}, nil
		}
		return Report{}, err
	}
	if segmentInfo.Expired(verifier.nowFn()) {
		verifier.log.Debug("segment expired before Verify")
		return Report{}, nil
	}

	randomIndex, err := GetRandomStripe(ctx, segmentInfo)
	if err != nil {
		return Report{}, err
	}

	var offlineNodes storj.NodeIDList
	var failedNodes metabase.Pieces
	var unknownNodes storj.NodeIDList
	containedNodes := make(map[int]storj.NodeID)
	sharesToAudit := make(map[int]Share)

	orderLimits, privateKey, cachedNodesInfo, err := verifier.orders.CreateAuditOrderLimits(ctx, segmentInfo, skip)
	if err != nil {
		if orders.ErrDownloadFailedNotEnoughPieces.Has(err) {
			mon.Counter("not_enough_shares_for_audit").Inc(1)
			mon.Counter("audit_not_enough_nodes_online").Inc(1)
			err = ErrNotEnoughShares.Wrap(err)
		}
		return Report{}, err
	}
	cachedNodesReputation := make(map[storj.NodeID]overlay.ReputationStatus, len(cachedNodesInfo))
	for id, info := range cachedNodesInfo {
		cachedNodesReputation[id] = info.Reputation
	}
	// Assigned to the end report, instead of having to set it in all of the following returns.
	defer func() { report.NodesReputation = cachedNodesReputation }()

	// NOTE offlineNodes will include disqualified nodes because they aren't in
	// the skip list
	offlineNodes = getOfflineNodes(segmentInfo, orderLimits, skip)
	if len(offlineNodes) > 0 {
		verifier.log.Debug("Verify: order limits not created for some nodes (offline/disqualified)",
			zap.Strings("Node IDs", offlineNodes.Strings()),
			zap.String("Segment", segmentInfoString(segment)))
	}

	// this will pass additional info to restored from trash event sent by underlying libuplink
	//nolint: revive
	//lint:ignore SA1029 this is a temporary solution
	ctx = context.WithValue(ctx, "restored_from_trash", map[string]string{
		"StreamID":       segment.StreamID.String(),
		"StreamPosition": strconv.Itoa(int(segment.Position.Encode())),
	})
	shares, err := verifier.DownloadShares(ctx, orderLimits, privateKey, cachedNodesInfo, randomIndex, segmentInfo.Redundancy.ShareSize)
	if err != nil {
		return Report{
			Offlines: offlineNodes,
		}, err
	}

	err = verifier.checkIfSegmentAltered(ctx, segmentInfo)
	if err != nil {
		if ErrSegmentDeleted.Has(err) {
			verifier.log.Debug("segment deleted during Verify")
			return Report{}, nil
		}
		if ErrSegmentModified.Has(err) {
			verifier.log.Debug("segment modified during Verify")
			return Report{}, nil
		}
		return Report{
			Offlines: offlineNodes,
		}, err
	}

	for pieceNum, share := range shares {
		if share.Error == nil {
			// no error -- share downloaded successfully
			sharesToAudit[pieceNum] = share
			continue
		}

		pieceID := orderLimits[pieceNum].Limit.PieceId
		errLogger := verifier.log.With(
			zap.Stringer("Node ID", share.NodeID),
			zap.String("Segment", segmentInfoString(segment)),
			zap.Stringer("Piece ID", pieceID),
			zap.Uint16("Placement", uint16(segmentInfo.Placement)),
			zap.Error(share.Error),
		)

		switch share.FailurePhase {
		case DialFailure:
			// dial failed -- offline node
			offlineNodes = append(offlineNodes, share.NodeID)
			errLogger.Debug("Verify: dial failed (offline)")
			continue

		case RequestFailure:
			if errs2.IsRPC(share.Error, rpcstatus.NotFound) {
				// missing share
				failedNodes = append(failedNodes, metabase.Piece{
					Number:      uint16(share.PieceNum),
					StorageNode: share.NodeID,
				})

				evs.Event("audit-piece-not-found",
					eventkit.Bytes("node-id", share.NodeID.Bytes()),
					eventkit.String("stream-id", segment.StreamID.String()),
					eventkit.Int64("stream-position", int64(segment.Position.Encode())),
					eventkit.Int64("piece-num", int64(share.PieceNum)),
					eventkit.Int64("placement", int64(segmentInfo.Placement)),
				)
				errLogger.Info("Verify: piece not found (audit failed)", zap.Int("piece-num", share.PieceNum))
				continue
			}

			if errs2.IsRPC(share.Error, rpcstatus.DeadlineExceeded) {
				// dial successful, but download timed out
				containedNodes[pieceNum] = share.NodeID
				errLogger.Info("Verify: download timeout (contained)")
				continue
			}

			// unknown error
			unknownNodes = append(unknownNodes, share.NodeID)
			errLogger.Info("Verify: unknown error (skipped)",
				zap.String("ErrorType", spew.Sprintf("%#+v", share.Error)))
		}
	}
	mon.IntVal("verify_shares_downloaded_successfully").Observe(int64(len(sharesToAudit)))

	required := segmentInfo.Redundancy.RequiredShares
	total := segmentInfo.Redundancy.TotalShares

	if len(sharesToAudit) < int(required) {
		mon.Counter("not_enough_shares_for_audit").Inc(1)
		// if we have reached this point, most likely something went wrong
		// like a network problem or a forgotten delete. Don't fail nodes.
		// We have an alert on this. Check the logs and see what happened.
		var errMsgDetails string
		if len(offlineNodes)+len(containedNodes) > len(sharesToAudit)+len(failedNodes)+len(unknownNodes) {
			mon.Counter("audit_suspected_network_problem").Inc(1)
			errMsgDetails = "(suspected network problem). "
		} else {
			mon.Counter("audit_not_enough_shares_acquired").Inc(1)
		}

		// The audit couldn't be performed, so we don't report failed nodes.
		report := Report{
			Offlines: offlineNodes,
			Unknown:  unknownNodes,
		}
		return report, ErrNotEnoughShares.New("%sgot: %d, required: %d, failed: %d, offline: %d, unknown: %d, contained: %d",
			errMsgDetails, len(sharesToAudit), required, len(failedNodes), len(offlineNodes), len(unknownNodes), len(containedNodes))
	}
	// ensure we get values, even if only zero values, so that redash can have an alert based on these
	mon.Counter("not_enough_shares_for_audit").Inc(0)
	mon.Counter("audit_not_enough_nodes_online").Inc(0)
	mon.Counter("audit_not_enough_shares_acquired").Inc(0)
	mon.Counter("audit_suspected_network_problem").Inc(0)

	pieceNums, _, err := auditShares(ctx, required, total, sharesToAudit)
	if err != nil {
		mon.Counter("could_not_verify_audit_shares").Inc(1)
		verifier.log.Error("could not verify shares", zap.String("Segment", segmentInfoString(segment)), zap.Error(err))
		return Report{
			Segment:  &segmentInfo,
			Fails:    failedNodes,
			Offlines: offlineNodes,
			Unknown:  unknownNodes,
		}, err
	}

	// ensure we get values, even if only zero values, so that redash can have an alert based on these
	mon.Counter("could_not_verify_audit_shares").Inc(0)

	for _, pieceNum := range pieceNums {
		verifier.log.Info("Verify: share data altered (audit failed)",
			zap.Stringer("Node ID", shares[pieceNum].NodeID),
			zap.String("Segment", segmentInfoString(segment)))
		failedNodes = append(failedNodes, metabase.Piece{
			StorageNode: shares[pieceNum].NodeID,
			Number:      uint16(pieceNum),
		})
	}

	successNodes := getSuccessNodes(ctx, shares, failedNodes, offlineNodes, unknownNodes, containedNodes)

	pendingAudits, err := createPendingAudits(ctx, containedNodes, segment)
	if err != nil {
		return Report{
			Segment:   &segmentInfo,
			Successes: successNodes,
			Fails:     failedNodes,
			Offlines:  offlineNodes,
			Unknown:   unknownNodes,
		}, err
	}

	return Report{
		Segment:       &segmentInfo,
		Successes:     successNodes,
		Fails:         failedNodes,
		Offlines:      offlineNodes,
		PendingAudits: pendingAudits,
		Unknown:       unknownNodes,
	}, nil
}

func segmentInfoString(segment Segment) string {
	return fmt.Sprintf("%s/%d",
		segment.StreamID.String(),
		segment.Position.Encode(),
	)
}

// DownloadShares downloads shares from the nodes where remote pieces are located.
func (verifier *Verifier) DownloadShares(ctx context.Context, limits []*pb.AddressedOrderLimit, piecePrivateKey storj.PiecePrivateKey, cachedNodesInfo map[storj.NodeID]overlay.NodeReputation, stripeIndex int32, shareSize int32) (shares map[int]Share, err error) {
	defer mon.Task()(&ctx)(&err)

	shares = make(map[int]Share, len(limits))
	ch := make(chan *Share, len(limits))

	for i, limit := range limits {
		if limit == nil {
			ch <- nil
			continue
		}

		var ipPort string
		node, ok := cachedNodesInfo[limit.Limit.StorageNodeId]
		if ok && node.LastIPPort != "" {
			ipPort = node.LastIPPort
		}

		go func(i int, limit *pb.AddressedOrderLimit) {
			share := verifier.GetShare(ctx, limit, piecePrivateKey, ipPort, stripeIndex, shareSize, i)
			ch <- &share
		}(i, limit)
	}

	for range limits {
		share := <-ch
		if share != nil {
			shares[share.PieceNum] = *share
		}
	}

	return shares, nil
}

// IdentifyContainedNodes returns the set of all contained nodes out of the
// holders of pieces in the given segment.
func (verifier *Verifier) IdentifyContainedNodes(ctx context.Context, segment Segment) (skipList map[storj.NodeID]bool, err error) {
	segmentInfo, err := verifier.metabase.GetSegmentByPositionForAudit(ctx, metabase.GetSegmentByPosition{
		StreamID: segment.StreamID,
		Position: segment.Position,
	})
	if err != nil {
		return nil, err
	}

	skipList = make(map[storj.NodeID]bool)
	for _, piece := range segmentInfo.Pieces {
		_, err := verifier.containment.Get(ctx, piece.StorageNode)
		if err != nil {
			if ErrContainedNotFound.Has(err) {
				continue
			}
			verifier.log.Error("can not determine if node is contained", zap.Stringer("node-id", piece.StorageNode), zap.Error(err))
			continue
		}
		skipList[piece.StorageNode] = true
	}
	return skipList, nil
}

// GetShare use piece store client to download shares from nodes.
func (verifier *Verifier) GetShare(ctx context.Context, limit *pb.AddressedOrderLimit, piecePrivateKey storj.PiecePrivateKey, cachedIPAndPort string, stripeIndex, shareSize int32, pieceNum int) (share Share) {
	defer mon.Task()(&ctx)(&share.Error)

	share.PieceNum = pieceNum
	share.NodeID = limit.GetLimit().StorageNodeId
	share.FailurePhase = DialFailure

	bandwidthMsgSize := shareSize

	// determines number of seconds allotted for receiving data from a storage node
	timedCtx := ctx
	if verifier.minBytesPerSecond > 0 {
		maxTransferTime := time.Duration(int64(time.Second) * int64(bandwidthMsgSize) / verifier.minBytesPerSecond.Int64())
		if maxTransferTime < verifier.minDownloadTimeout {
			maxTransferTime = verifier.minDownloadTimeout
		}
		var cancel func()
		timedCtx, cancel = context.WithTimeout(ctx, maxTransferTime)
		defer cancel()
	}

	targetNodeID := limit.GetLimit().StorageNodeId
	log := verifier.log.Named(targetNodeID.String())
	var ps *piecestore.Client
	var err error

	// if cached IP is given, try connecting there first
	if cachedIPAndPort != "" {
		nodeAddr := storj.NodeURL{
			ID:      targetNodeID,
			Address: cachedIPAndPort,
		}
		ps, err = piecestore.Dial(rpcpool.WithForceDial(timedCtx), verifier.dialer, nodeAddr, piecestore.DefaultConfig)
		if err != nil {
			log.Debug("failed to connect to audit target node at cached IP", zap.String("cached-ip-and-port", cachedIPAndPort), zap.Error(err))
		}
	}

	// if no cached IP was given, or connecting to cached IP failed, use node address
	if ps == nil {
		nodeAddr := storj.NodeURL{
			ID:      targetNodeID,
			Address: limit.GetStorageNodeAddress().Address,
		}
		ps, err = piecestore.Dial(rpcpool.WithForceDial(timedCtx), verifier.dialer, nodeAddr, piecestore.DefaultConfig)
		if err != nil {
			share.Error = Error.Wrap(err)
			return share
		}
	}

	share.FailurePhase = RequestFailure
	defer func() {
		err := ps.Close()
		if err != nil {
			verifier.log.Error("audit verifier failed to close conn to node: %+v", zap.Error(err))
		}
	}()

	offset := int64(shareSize) * int64(stripeIndex)

	downloader, err := ps.Download(timedCtx, limit.GetLimit(), piecePrivateKey, offset, int64(shareSize))
	if err != nil {
		share.Error = err
		return share
	}

	buf := make([]byte, shareSize)
	_, err = io.ReadFull(downloader, buf)
	closeErr := downloader.Close()
	if err != nil || closeErr != nil {
		if errors.Is(err, io.ErrClosedPipe) {
			// in some circumstances, this can be returned from the piecestore
			// when the peer returned a different error. The downloader gets
			// marked as being closed, even though we haven't closed it from
			// this side, and ErrClosedPipe gets returned on the next Read
			// instead of the actual error. We'll get the real error from
			// downloader.Close().
			err = nil
		}
		share.Error = errs.Combine(err, closeErr)
		return share
	}
	share.Data = buf
	share.FailurePhase = NoFailure

	return share
}

// checkIfSegmentAltered checks if oldSegment has been altered since it was selected for audit.
func (verifier *Verifier) checkIfSegmentAltered(
	ctx context.Context, oldSegment metabase.SegmentForAudit,
) (err error) {
	defer mon.Task()(&ctx)(&err)

	if verifier.OnTestingCheckSegmentAlteredHook != nil {
		verifier.OnTestingCheckSegmentAlteredHook()
	}

	newSegment, err := verifier.metabase.GetSegmentByPositionForAudit(ctx, metabase.GetSegmentByPosition{
		StreamID: oldSegment.StreamID,
		Position: oldSegment.Position,
	})
	if err != nil {
		if metabase.ErrSegmentNotFound.Has(err) {
			return ErrSegmentDeleted.New("StreamID: %q Position: %d", oldSegment.StreamID.String(), oldSegment.Position.Encode())
		}
		return err
	}

	if !oldSegment.Pieces.Equal(newSegment.Pieces) {
		return ErrSegmentModified.New("StreamID: %q Position: %d", oldSegment.StreamID.String(), oldSegment.Position.Encode())
	}
	return nil
}

// SetNow allows tests to have the server act as if the current time is whatever they want.
func (verifier *Verifier) SetNow(nowFn func() time.Time) {
	verifier.nowFn = nowFn
}

// auditShares takes the downloaded shares and uses infectious's Correct function to check that they
// haven't been altered. auditShares returns a slice containing the piece numbers of altered shares,
// and a slice of the corrected shares.
func auditShares(ctx context.Context, required, total int16, originals map[int]Share) (pieceNums []int, corrected []eestream.Share, err error) {
	defer mon.Task()(&ctx)(&err)
	f, err := eestream.NewFEC(int(required), int(total))
	if err != nil {
		return nil, nil, err
	}

	copies, err := makeCopies(ctx, originals)
	if err != nil {
		return nil, nil, err
	}

	err = f.Correct(copies)
	if err != nil {
		return nil, nil, err
	}

	for _, share := range copies {
		if !bytes.Equal(originals[share.Number].Data, share.Data) {
			pieceNums = append(pieceNums, share.Number)
		}
	}
	return pieceNums, copies, nil
}

// makeCopies takes in a map of audit Shares and deep copies their data to a slice of eestream Shares.
func makeCopies(ctx context.Context, originals map[int]Share) (copies []eestream.Share, err error) {
	defer mon.Task()(&ctx)(&err)
	copies = make([]eestream.Share, 0, len(originals))
	for _, original := range originals {
		copies = append(copies, eestream.Share{
			Data:   append([]byte{}, original.Data...),
			Number: original.PieceNum})
	}
	return copies, nil
}

// getOfflineNodes returns those storage nodes from the segment which have no
// order limit nor are skipped.
func getOfflineNodes(
	segment metabase.SegmentForAudit, limits []*pb.AddressedOrderLimit, skip map[storj.NodeID]bool,
) storj.NodeIDList {
	var offlines storj.NodeIDList

	nodesWithLimit := make(map[storj.NodeID]bool, len(limits))
	for _, limit := range limits {
		if limit != nil {
			nodesWithLimit[limit.GetLimit().StorageNodeId] = true
		}
	}

	for _, piece := range segment.Pieces {
		if !nodesWithLimit[piece.StorageNode] && !skip[piece.StorageNode] {
			offlines = append(offlines, piece.StorageNode)
		}
	}

	return offlines
}

// getSuccessNodes uses the failed nodes, offline nodes and contained nodes arrays to determine which nodes passed the audit.
func getSuccessNodes(ctx context.Context, shares map[int]Share, failedNodes metabase.Pieces, offlineNodes, unknownNodes storj.NodeIDList, containedNodes map[int]storj.NodeID) (successNodes storj.NodeIDList) {
	defer mon.Task()(&ctx)(nil)
	fails := make(map[storj.NodeID]bool)
	for _, fail := range failedNodes {
		fails[fail.StorageNode] = true
	}
	for _, offline := range offlineNodes {
		fails[offline] = true
	}
	for _, unknown := range unknownNodes {
		fails[unknown] = true
	}
	for _, contained := range containedNodes {
		fails[contained] = true
	}

	for _, share := range shares {
		if !fails[share.NodeID] {
			successNodes = append(successNodes, share.NodeID)
		}
	}

	return successNodes
}

func createPendingAudits(ctx context.Context, containedNodes map[int]storj.NodeID, segment Segment) (pending []*ReverificationJob, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(containedNodes) == 0 {
		return nil, nil
	}

	pending = make([]*ReverificationJob, 0, len(containedNodes))
	for pieceNum, nodeID := range containedNodes {
		pending = append(pending, &ReverificationJob{
			Locator: PieceLocator{
				NodeID:   nodeID,
				StreamID: segment.StreamID,
				Position: segment.Position,
				PieceNum: pieceNum,
			},
		})
	}

	return pending, nil
}

// GetRandomStripe takes a segment and returns a random stripe index within that segment.
func GetRandomStripe(ctx context.Context, segment metabase.SegmentForAudit) (index int32, err error) {
	defer mon.Task()(&ctx)(&err)

	// the last segment could be smaller than stripe size
	if segment.EncryptedSize < segment.Redundancy.StripeSize() {
		return 0, nil
	}

	var src cryptoSource
	rnd := rand.New(src)
	numStripes := segment.Redundancy.StripeCount(segment.EncryptedSize)
	randomStripeIndex := rnd.Int31n(numStripes)

	return randomStripeIndex, nil
}

func recordStats(report Report, totalPieces int, verifyErr error) {
	// If an audit was able to complete without auditing any nodes, that means
	// the segment is expired or has been altered.
	if verifyErr == nil && len(report.Successes) == 0 {
		return
	}

	numOffline := len(report.Offlines)
	numSuccessful := len(report.Successes)
	numFailed := len(report.Fails)
	numContained := len(report.PendingAudits)
	numUnknown := len(report.Unknown)

	totalAudited := numSuccessful + numFailed + numOffline + numContained
	auditedPercentage := float64(totalAudited) / float64(totalPieces)
	offlinePercentage := float64(0)
	successfulPercentage := float64(0)
	failedPercentage := float64(0)
	containedPercentage := float64(0)
	unknownPercentage := float64(0)
	if totalAudited > 0 {
		offlinePercentage = float64(numOffline) / float64(totalAudited)
		successfulPercentage = float64(numSuccessful) / float64(totalAudited)
		failedPercentage = float64(numFailed) / float64(totalAudited)
		containedPercentage = float64(numContained) / float64(totalAudited)
		unknownPercentage = float64(numUnknown) / float64(totalAudited)
	}

	tags := []monkit.SeriesTag{}
	if report.Segment != nil {
		tags = append(tags, monkit.NewSeriesTag("placement", strconv.FormatUint(uint64(report.Segment.Placement), 10)))
	}

	mon.Meter("audit_success_nodes_global", tags...).Mark(numSuccessful)
	mon.Meter("audit_fail_nodes_global", tags...).Mark(numFailed)
	mon.Meter("audit_offline_nodes_global", tags...).Mark(numOffline)
	mon.Meter("audit_contained_nodes_global", tags...).Mark(numContained)
	mon.Meter("audit_unknown_nodes_global", tags...).Mark(numUnknown)
	mon.Meter("audit_total_nodes_global", tags...).Mark(totalAudited)
	mon.Meter("audit_total_pointer_nodes_global", tags...).Mark(totalPieces)

	mon.IntVal("audit_success_nodes", tags...).Observe(int64(numSuccessful))
	mon.IntVal("audit_fail_nodes", tags...).Observe(int64(numFailed))
	mon.IntVal("audit_offline_nodes", tags...).Observe(int64(numOffline))
	mon.IntVal("audit_contained_nodes", tags...).Observe(int64(numContained))
	mon.IntVal("audit_unknown_nodes", tags...).Observe(int64(numUnknown))
	mon.IntVal("audit_total_nodes", tags...).Observe(int64(totalAudited))
	mon.IntVal("audit_total_pointer_nodes", tags...).Observe(int64(totalPieces))
	mon.FloatVal("audited_percentage", tags...).Observe(auditedPercentage)
	mon.FloatVal("audit_offline_percentage", tags...).Observe(offlinePercentage)
	mon.FloatVal("audit_successful_percentage", tags...).Observe(successfulPercentage)
	mon.FloatVal("audit_failed_percentage", tags...).Observe(failedPercentage)
	mon.FloatVal("audit_contained_percentage", tags...).Observe(containedPercentage)
	mon.FloatVal("audit_unknown_percentage", tags...).Observe(unknownPercentage)
}
