// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	monkit "github.com/spacemonkeygo/monkit/v3"
	"go.uber.org/zap"

	"storj.io/common/identity"
	"storj.io/common/macaroon"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/internalpb"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
)

func calculateSpaceUsed(segmentSize int64, numberOfPieces int, rs storj.RedundancyScheme) (totalStored int64) {
	pieceSize := segmentSize / int64(rs.RequiredShares)
	return pieceSize * int64(numberOfPieces)
}

// BeginSegment begins segment uploading.
func (endpoint *Endpoint) BeginSegment(ctx context.Context, req *pb.SegmentBeginRequest) (resp *pb.SegmentBeginResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	return endpoint.beginSegment(ctx, req, false)
}

func (endpoint *Endpoint) beginSegment(ctx context.Context, req *pb.SegmentBeginRequest, objectJustCreated bool) (resp *pb.SegmentBeginResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())

	peer, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		// N.B. jeff thinks this is a bad idea but jt convinced him
		return nil, rpcstatus.Errorf(rpcstatus.Unauthenticated, "unable to get peer identity: %w", err)
	}

	streamID, err := endpoint.unmarshalSatStreamID(ctx, req.StreamId)
	if err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.InvalidArgument, err)
	}

	keyInfo, err := endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:            macaroon.ActionWrite,
		Bucket:        streamID.Bucket,
		EncryptedPath: streamID.EncryptedObjectKey,
		Time:          time.Now(),
	}, console.RateLimitPutNoError)
	if err != nil {
		return nil, err
	}
	endpoint.usageTracking(keyInfo, req.Header, fmt.Sprintf("%T", req))

	// no need to validate streamID fields because it was validated during BeginObject

	if req.Position.Index < 0 {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, "segment index must be greater then 0")
	}

	objectJustCreated = objectJustCreated || !streamID.MultipartObject
	if !objectJustCreated {
		// we need check limits only if object wasn't just created,
		// begin object is checking limits on it' own
		if err := endpoint.checkUploadLimits(ctx, keyInfo); err != nil {
			return nil, err
		}
	}

	placement := endpoint.placement[storj.PlacementConstraint(streamID.Placement)]
	config := endpoint.config
	rsParams := config.RS.Override(placement.EC)
	defaultRedundancy := storj.RedundancyScheme{
		Algorithm:      storj.ReedSolomon,
		RequiredShares: int16(rsParams.Min),
		RepairShares:   int16(rsParams.Repair),
		OptimalShares:  int16(rsParams.Success),
		TotalShares:    int16(rsParams.Total),
		ShareSize:      rsParams.ErasureShareSize.Int32(),
	}

	maxPieceSize := defaultRedundancy.PieceSize(req.MaxOrderLimit)

	nodes, err := endpoint.overlay.FindStorageNodesForUpload(ctx, overlay.FindStorageNodesRequest{
		RequestedCount: int(defaultRedundancy.TotalShares),
		Placement:      storj.PlacementConstraint(streamID.Placement),
		Requester:      peer.ID,
	})

	for _, node := range nodes {
		endpoint.nodeSelectionStats.IncrementInitial(node.ID)
	}

	if err != nil {
		if overlay.ErrNotEnoughNodes.Has(err) {
			return nil, rpcstatus.Error(rpcstatus.FailedPrecondition, err.Error())
		}
		return nil, endpoint.ConvertKnownErrWithMessage(err, "internal error")
	}

	var (
		rootPieceID     storj.PieceID
		addressedLimits []*pb.AddressedOrderLimit
		piecePrivateKey storj.PiecePrivateKey
	)

	bucket := metabase.BucketLocation{ProjectID: keyInfo.ProjectID, BucketName: metabase.BucketName(streamID.Bucket)}

	if objectJustCreated && req.LiteRequest {
		rootPieceID, addressedLimits, piecePrivateKey, err = endpoint.orders.CreateLitePutOrderLimits(ctx, bucket, nodes, streamID.ExpirationDate, maxPieceSize)
		if err != nil {
			return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to create order limits")
		}
	} else {
		rootPieceID, addressedLimits, piecePrivateKey, err = endpoint.orders.CreatePutOrderLimits(ctx, bucket, nodes, streamID.ExpirationDate, maxPieceSize)
		if err != nil {
			return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to create order limits")
		}
	}

	id, err := uuid.FromBytes(streamID.StreamId)
	if err != nil {
		return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to parse stream id")
	}

	pieces := metabase.Pieces{}
	for i, limit := range addressedLimits {
		pieces = append(pieces, metabase.Piece{
			Number:      uint16(i),
			StorageNode: limit.Limit.StorageNodeId,
		})
		if placement.CohortNames != nil {
			addressedLimits[i].Tags = make(map[string][]byte, len(placement.CohortNames))
			for name, val := range placement.CohortNames {
				addressedLimits[i].Tags[name] = val(*nodes[i])
			}
		}
	}
	err = endpoint.metabase.BeginSegment(ctx, metabase.BeginSegment{
		ObjectStream: metabase.ObjectStream{
			ProjectID:  keyInfo.ProjectID,
			BucketName: metabase.BucketName(streamID.Bucket),
			ObjectKey:  metabase.ObjectKey(streamID.EncryptedObjectKey),
			StreamID:   id,
			Version:    metabase.Version(streamID.Version),
		},
		Position: metabase.SegmentPosition{
			Part:  uint32(req.Position.PartNumber),
			Index: uint32(req.Position.Index),
		},
		RootPieceID:         rootPieceID,
		Pieces:              pieces,
		ObjectExistsChecked: objectJustCreated,
	})
	if err != nil {
		return nil, endpoint.ConvertMetabaseErr(err)
	}

	redundancyScheme := endpoint.getRSProto(storj.PlacementConstraint(streamID.Placement))

	segmentID, err := endpoint.packSegmentID(ctx, &internalpb.SegmentID{
		StreamId:            streamID,
		PartNumber:          req.Position.PartNumber,
		Index:               req.Position.Index,
		OriginalOrderLimits: addressedLimits,
		RootPieceId:         rootPieceID,
		CreationDate:        time.Now(),
		RedundancyScheme:    redundancyScheme,
	})
	if err != nil {
		return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to create segment id")
	}

	endpoint.log.Debug("Segment Upload", zap.Stringer("Project ID", keyInfo.ProjectID), zap.String("operation", "put"), zap.String("type", "remote"))
	mon.Meter("req_put_remote").Mark(1)

	var cohortRequirements *pb.CohortRequirements
	if placement.CohortRequirements != nil {
		cohortRequirements = placement.CohortRequirements.ToProto()
	}

	return &pb.SegmentBeginResponse{
		SegmentId:          segmentID,
		AddressedLimits:    addressedLimits,
		PrivateKey:         piecePrivateKey,
		RedundancyScheme:   redundancyScheme,
		CohortRequirements: cohortRequirements,
	}, nil
}

// RetryBeginSegmentPieces replaces put order limits for failed piece uploads.
func (endpoint *Endpoint) RetryBeginSegmentPieces(ctx context.Context, req *pb.RetryBeginSegmentPiecesRequest) (resp *pb.RetryBeginSegmentPiecesResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())

	peer, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		// N.B. jeff thinks this is a bad idea but jt convinced him
		return nil, rpcstatus.Errorf(rpcstatus.Unauthenticated, "unable to get peer identity: %w", err)
	}

	segmentID, err := endpoint.unmarshalSatSegmentID(ctx, req.SegmentId)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	keyInfo, err := endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:            macaroon.ActionWrite,
		Bucket:        segmentID.StreamId.Bucket,
		EncryptedPath: segmentID.StreamId.EncryptedObjectKey,
		Time:          time.Now(),
	}, console.RateLimitPutNoError)
	if err != nil {
		return nil, err
	}
	endpoint.usageTracking(keyInfo, req.Header, fmt.Sprintf("%T", req))

	if len(req.RetryPieceNumbers) == 0 {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, "piece numbers to exchange cannot be empty")
	}

	retryingPieceNumberSet := make(map[int32]struct{}, len(req.RetryPieceNumbers))
	for _, pieceNumber := range req.RetryPieceNumbers {
		if pieceNumber < 0 || int(pieceNumber) >= len(segmentID.OriginalOrderLimits) {
			endpoint.log.Debug("piece number is out of range",
				zap.Int32("piece number", pieceNumber),
				zap.Int("redundancy total", len(segmentID.OriginalOrderLimits)),
				zap.Stringer("Segment ID", req.SegmentId),
			)
			return nil, rpcstatus.Errorf(rpcstatus.InvalidArgument, "piece number %d must be within range [0,%d]", pieceNumber, len(segmentID.OriginalOrderLimits)-1)
		}
		if _, ok := retryingPieceNumberSet[pieceNumber]; ok {
			endpoint.log.Debug("piece number is duplicated",
				zap.Int32("piece number", pieceNumber),
				zap.Stringer("Segment ID", req.SegmentId),
			)
			return nil, rpcstatus.Errorf(rpcstatus.InvalidArgument, "piece number %d is duplicated", pieceNumber)
		}
		retryingPieceNumberSet[pieceNumber] = struct{}{}
	}

	if err := endpoint.checkUploadLimits(ctx, keyInfo); err != nil {
		return nil, err
	}

	// Find a new set of storage nodes, excluding any already represented in
	// the current list of order limits.
	// TODO: It's possible that a node gets reused across multiple calls to RetryBeginSegmentPieces.
	// We use this slice to excluded the already used nodes to store part of the pieces and the ones
	// that failed to store a piece, which are the ones that we are going to retry. The method, that
	// receives this list, distinguishes between both receiving them in two separated slices, but we
	// use one because we need to perform a considerable amount of changes before we can split them.
	// See https://github.com/storj/storj/issues/7675
	excludedIDs := make([]storj.NodeID, 0, len(segmentID.OriginalOrderLimits))
	dedicatedSuccessTracker := endpoint.successTrackers.GetDedicatedTracker(peer.ID)
	globalSuccessTracker := endpoint.successTrackers.GetGlobalTracker()
	isTrusted := endpoint.trustedUplinks.IsTrusted(peer.ID)
	for pieceNumber, orderLimit := range segmentID.OriginalOrderLimits {
		excludedIDs = append(excludedIDs, orderLimit.Limit.StorageNodeId)
		if _, found := retryingPieceNumberSet[int32(pieceNumber)]; found {
			endpoint.updateTrackers(ctx, dedicatedSuccessTracker, globalSuccessTracker, isTrusted, orderLimit.Limit.StorageNodeId, false)
		}
	}

	nodes, err := endpoint.overlay.FindStorageNodesForUpload(ctx, overlay.FindStorageNodesRequest{
		RequestedCount: len(req.RetryPieceNumbers),
		Placement:      storj.PlacementConstraint(segmentID.StreamId.Placement),
		ExcludedIDs:    excludedIDs,
	})
	if err != nil {
		if overlay.ErrNotEnoughNodes.Has(err) {
			return nil, rpcstatus.Error(rpcstatus.FailedPrecondition, err.Error())
		}
		return nil, endpoint.ConvertKnownErrWithMessage(err, err.Error())
	}

	for _, node := range nodes {
		endpoint.nodeSelectionStats.IncrementRetry(node.ID)
	}

	addressedLimits, err := endpoint.orders.ReplacePutOrderLimits(ctx, segmentID.RootPieceId, segmentID.OriginalOrderLimits, nodes, req.RetryPieceNumbers)
	if err != nil {
		return nil, endpoint.ConvertKnownErrWithMessage(err, "internal error")
	}

	placement := endpoint.placement[storj.PlacementConstraint(segmentID.StreamId.Placement)]
	if placement.CohortNames != nil {
		for i, piecenum := range req.RetryPieceNumbers {
			addressedLimits[piecenum].Tags = make(map[string][]byte, len(placement.CohortNames))
			for name, val := range placement.CohortNames {
				addressedLimits[piecenum].Tags[name] = val(*nodes[i])
			}
		}
	}

	segmentID.OriginalOrderLimits = addressedLimits

	amendedSegmentID, err := endpoint.packSegmentID(ctx, segmentID)
	if err != nil {
		return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to create segment id")
	}

	endpoint.log.Debug("Segment Upload Piece Retry", zap.Stringer("Project ID", keyInfo.ProjectID), zap.String("operation", "put"), zap.String("type", "remote"))

	return &pb.RetryBeginSegmentPiecesResponse{
		SegmentId:       amendedSegmentID,
		AddressedLimits: addressedLimits,
	}, nil
}

func (endpoint *Endpoint) updateTrackers(ctx context.Context, dedicatedSuccessTracker, globalSuccessTracker SuccessTracker, isTrusted bool, nodeID storj.NodeID, success bool) {
	if dedicatedSuccessTracker != nil {
		dedicatedSuccessTracker.Increment(nodeID, success)
	}

	if endpoint.config.AlwaysUpdateGlobalTracker || dedicatedSuccessTracker == nil {
		globalSuccessTracker.Increment(nodeID, success)
	}

	if isTrusted {
		endpoint.failureTracker.Increment(nodeID, success)
	}
}

// CommitSegment commits segment after uploading.
func (endpoint *Endpoint) CommitSegment(ctx context.Context, req *pb.SegmentCommitRequest) (resp *pb.SegmentCommitResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())

	peer, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		// N.B. jeff thinks this is a bad idea but jt convinced him
		return nil, rpcstatus.Errorf(rpcstatus.Unauthenticated, "unable to get peer identity: %w", err)
	}

	segmentID, err := endpoint.unmarshalSatSegmentID(ctx, req.SegmentId)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	streamID := segmentID.StreamId

	keyInfo, err := endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:            macaroon.ActionWrite,
		Bucket:        streamID.Bucket,
		EncryptedPath: streamID.EncryptedObjectKey,
		Time:          time.Now(),
	}, console.RateLimitPutNoError)
	if err != nil {
		return nil, err
	}
	endpoint.usageTracking(keyInfo, req.Header, fmt.Sprintf("%T", req))

	// cheap basic verification
	rsParam := segmentID.RedundancyScheme
	if rsParam == nil {
		rsParam = endpoint.getRSProto(storj.PlacementConstraint(streamID.Placement))
	}
	if numResults := len(req.UploadResult); numResults < int(rsParam.GetSuccessThreshold()) {
		endpoint.log.Debug("the results of uploaded pieces for the segment is below the redundancy optimal threshold",
			zap.Int("upload pieces results", numResults),
			zap.Int32("redundancy optimal threshold", rsParam.GetSuccessThreshold()),
			zap.Stringer("Segment ID", req.SegmentId),
		)
		return nil, rpcstatus.Errorf(rpcstatus.InvalidArgument,
			"the number of results of uploaded pieces (%d) is below the optimal threshold (%d)",
			numResults, rsParam.GetSuccessThreshold(),
		)
	}

	rs := storj.RedundancyScheme{
		Algorithm:      storj.RedundancyAlgorithm(rsParam.Type),
		RequiredShares: int16(rsParam.MinReq),
		RepairShares:   int16(rsParam.RepairThreshold),
		OptimalShares:  int16(rsParam.SuccessThreshold),
		TotalShares:    int16(rsParam.Total),
		ShareSize:      rsParam.ErasureShareSize,
	}

	err = endpoint.pointerVerification.VerifySizes(ctx, rs, req.SizeEncryptedData, req.UploadResult)
	if err != nil {
		endpoint.log.Debug("piece sizes are invalid", zap.Error(err))
		return nil, rpcstatus.Errorf(rpcstatus.InvalidArgument, "piece sizes are invalid: %v", err)
	}

	// extract the original order limits
	originalLimits := make([]*pb.OrderLimit, len(segmentID.OriginalOrderLimits))
	for i, orderLimit := range segmentID.OriginalOrderLimits {
		originalLimits[i] = orderLimit.Limit
	}

	// verify the piece upload results
	validPieces, invalidPieces, err := endpoint.pointerVerification.SelectValidPieces(ctx, peer, req.UploadResult, originalLimits)
	if err != nil {
		endpoint.log.Debug("pointer verification failed", zap.Error(err))
		return nil, rpcstatus.Errorf(rpcstatus.InvalidArgument, "pointer verification failed: %s", err)
	}

	if len(validPieces) < int(rs.OptimalShares) {
		endpoint.log.Debug("Number of valid pieces is less than the success threshold",
			zap.Int("totalReceivedPieces", len(req.UploadResult)),
			zap.Int("validPieces", len(validPieces)),
			zap.Int("invalidPieces", len(invalidPieces)),
			zap.Int("successThreshold", int(rs.OptimalShares)),
		)

		errMsg := fmt.Sprintf("Number of valid pieces (%d) is less than the success threshold (%d). Found %d invalid pieces",
			len(validPieces),
			rs.OptimalShares,
			len(invalidPieces),
		)
		if len(invalidPieces) > 0 {
			errMsg += ". Invalid Pieces:"
			for _, p := range invalidPieces {
				errMsg += fmt.Sprintf("\nNodeID: %v, PieceNum: %d, Reason: %s",
					p.NodeID, p.PieceNum, p.Reason,
				)
			}
		}
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, errMsg)
	}

	pieces := metabase.Pieces{}
	for _, result := range validPieces {
		pieces = append(pieces, metabase.Piece{
			Number:      uint16(result.PieceNum),
			StorageNode: result.NodeId,
		})
	}

	id, err := uuid.FromBytes(streamID.StreamId)
	if err != nil {
		return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to parse stream id")
	}

	var expiresAt *time.Time
	if !streamID.ExpirationDate.IsZero() {
		expiresAt = &streamID.ExpirationDate
	}
	var maxCommitDelay *time.Duration
	if _, ok := endpoint.config.TestingProjectsWithCommitDelay[keyInfo.ProjectID]; ok {
		maxCommitDelay = &endpoint.config.TestingMaxCommitDelay
	}

	mbCommitSegment := metabase.CommitSegment{
		ObjectStream: metabase.ObjectStream{
			ProjectID:  keyInfo.ProjectID,
			BucketName: metabase.BucketName(streamID.Bucket),
			ObjectKey:  metabase.ObjectKey(streamID.EncryptedObjectKey),
			StreamID:   id,
			Version:    metabase.Version(streamID.Version),
		},
		ExpiresAt:         expiresAt,
		EncryptedKey:      req.EncryptedKey,
		EncryptedKeyNonce: req.EncryptedKeyNonce[:],

		EncryptedSize: int32(req.SizeEncryptedData), // TODO incompatible types int32 vs int64
		PlainSize:     int32(req.PlainSize),         // TODO incompatible types int32 vs int64

		EncryptedETag: req.EncryptedETag,

		Position: metabase.SegmentPosition{
			Part:  uint32(segmentID.PartNumber),
			Index: uint32(segmentID.Index),
		},
		RootPieceID: segmentID.RootPieceId,
		Redundancy:  rs,
		Pieces:      pieces,
		Placement:   storj.PlacementConstraint(streamID.Placement),

		SkipPendingObject: !streamID.MultipartObject && endpoint.config.isNoPendingObjectUploadEnabled(keyInfo.ProjectID),

		MaxCommitDelay:      maxCommitDelay,
		TestingUseMutations: endpoint.config.TestingCommitSegmentUseMutations,
	}

	err = endpoint.validateRemoteSegment(ctx, mbCommitSegment, originalLimits)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	if err := endpoint.checkUploadLimits(ctx, keyInfo); err != nil {
		return nil, err
	}

	segmentSize := req.SizeEncryptedData
	totalStored := calculateSpaceUsed(segmentSize, len(pieces), rs)

	// ToDo: Replace with hash & signature validation
	// Ensure neither uplink or storage nodes are cheating on us

	// We cannot have more redundancy than total/min
	if float64(totalStored) > (float64(segmentSize)/float64(rs.RequiredShares))*float64(rs.TotalShares) {
		endpoint.log.Debug("data size mismatch",
			zap.Int64("segment", segmentSize),
			zap.Int64("pieces", totalStored),
			zap.Int16("redundancy minimum requested", rs.RequiredShares),
			zap.Int16("redundancy total", rs.TotalShares),
		)
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, "mismatched segment size and piece usage")
	}

	err = endpoint.metabase.CommitSegment(ctx, mbCommitSegment)
	if err != nil {
		return nil, endpoint.ConvertMetabaseErr(err)
	}

	endpoint.addSegmentToUploadLimits(ctx, keyInfo, segmentSize)

	// increment our counters in the success tracker appropriate to the committing uplink
	{
		dedicatedTracker := endpoint.successTrackers.GetDedicatedTracker(peer.ID)
		globalTracker := endpoint.successTrackers.GetGlobalTracker()
		isTrusted := endpoint.trustedUplinks.IsTrusted(peer.ID)
		validPieceSet := make(map[storj.NodeID]struct{}, len(validPieces))
		for _, piece := range validPieces {
			endpoint.updateTrackers(ctx, dedicatedTracker, globalTracker, isTrusted, piece.NodeId, true)
			validPieceSet[piece.NodeId] = struct{}{}
		}
		for _, limit := range originalLimits {
			if _, ok := validPieceSet[limit.StorageNodeId]; !ok {
				endpoint.updateTrackers(ctx, dedicatedTracker, globalTracker, isTrusted, limit.StorageNodeId, false)
			}
		}
	}

	// note: we collect transfer stats in CommitSegment instead because in BeginSegment
	// they would always be MaxSegmentSize (64MiB)
	endpoint.versionCollector.collectTransferStats(req.Header.UserAgent, upload, int(req.PlainSize))

	// Track piece-level telemetry for garbage discrepancy analysis
	placement := storj.PlacementConstraint(streamID.Placement)
	placementTag := monkit.NewSeriesTag("placement", strconv.FormatInt(int64(placement), 10))
	mon.IntVal("segment_commit_pieces_successful", placementTag).Observe(int64(len(pieces)))
	mon.IntVal("segment_commit_pieces_received", placementTag).Observe(int64(len(req.UploadResult)))
	mon.IntVal("segment_commit_pieces_invalid", placementTag).Observe(int64(len(invalidPieces)))

	mon.Meter("req_commit_segment").Mark(1)

	return &pb.SegmentCommitResponse{
		SuccessfulPieces: int32(len(pieces)),
	}, nil
}

// MakeInlineSegment makes inline segment on satellite.
func (endpoint *Endpoint) MakeInlineSegment(ctx context.Context, req *pb.SegmentMakeInlineRequest) (resp *pb.SegmentMakeInlineResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())

	streamID, err := endpoint.unmarshalSatStreamID(ctx, req.StreamId)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	keyInfo, err := endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:            macaroon.ActionWrite,
		Bucket:        streamID.Bucket,
		EncryptedPath: streamID.EncryptedObjectKey,
		Time:          time.Now(),
	}, console.RateLimitPut)
	if err != nil {
		return nil, err
	}
	endpoint.usageTracking(keyInfo, req.Header, fmt.Sprintf("%T", req))

	if req.Position.Index < 0 {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, "segment index must be greater then 0")
	}

	inlineUsed := int64(len(req.EncryptedInlineData))
	if inlineUsed > endpoint.encInlineSegmentSize {
		return nil, rpcstatus.Errorf(rpcstatus.InvalidArgument, "inline segment size cannot be larger than %s", endpoint.config.MaxInlineSegmentSize)
	}

	id, err := uuid.FromBytes(streamID.StreamId)
	if err != nil {
		return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to parse stream id")
	}

	if err := endpoint.checkUploadLimits(ctx, keyInfo); err != nil {
		return nil, err
	}

	var expiresAt *time.Time
	if !streamID.ExpirationDate.IsZero() {
		expiresAt = &streamID.ExpirationDate
	}

	var maxCommitDelay *time.Duration
	if _, ok := endpoint.config.TestingProjectsWithCommitDelay[keyInfo.ProjectID]; ok {
		maxCommitDelay = &endpoint.config.TestingMaxCommitDelay
	}

	err = endpoint.metabase.CommitInlineSegment(ctx, metabase.CommitInlineSegment{
		ObjectStream: metabase.ObjectStream{
			ProjectID:  keyInfo.ProjectID,
			BucketName: metabase.BucketName(streamID.Bucket),
			ObjectKey:  metabase.ObjectKey(streamID.EncryptedObjectKey),
			StreamID:   id,
			Version:    metabase.Version(streamID.Version),
		},
		ExpiresAt:         expiresAt,
		EncryptedKey:      req.EncryptedKey,
		EncryptedKeyNonce: req.EncryptedKeyNonce.Bytes(),

		Position: metabase.SegmentPosition{
			Part:  uint32(req.Position.PartNumber),
			Index: uint32(req.Position.Index),
		},

		PlainSize:     int32(req.PlainSize), // TODO incompatible types int32 vs int64
		EncryptedETag: req.EncryptedETag,

		InlineData: req.EncryptedInlineData,

		SkipPendingObject: !streamID.MultipartObject && endpoint.config.isNoPendingObjectUploadEnabled(keyInfo.ProjectID),

		MaxCommitDelay: maxCommitDelay,
	})
	if err != nil {
		return nil, endpoint.ConvertMetabaseErr(err)
	}

	bucket := metabase.BucketLocation{ProjectID: keyInfo.ProjectID, BucketName: metabase.BucketName(streamID.Bucket)}
	err = endpoint.orders.UpdatePutInlineOrder(ctx, bucket, inlineUsed)
	if err != nil {
		return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to update PUT inline order")
	}

	endpoint.addSegmentToUploadLimits(ctx, keyInfo, inlineUsed)

	endpoint.versionCollector.collectTransferStats(req.Header.UserAgent, upload, int(req.PlainSize))

	endpoint.log.Debug("Inline Segment Upload", zap.Stringer("Project ID", keyInfo.ProjectID), zap.String("operation", "put"), zap.String("type", "inline"))
	mon.Meter("req_put_inline").Mark(1)

	return &pb.SegmentMakeInlineResponse{}, nil
}

// ListSegments list object segments.
func (endpoint *Endpoint) ListSegments(ctx context.Context, req *pb.SegmentListRequest) (resp *pb.SegmentListResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())

	streamID, err := endpoint.unmarshalSatStreamID(ctx, req.StreamId)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	keyInfo, err := endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:            macaroon.ActionRead,
		Bucket:        streamID.Bucket,
		EncryptedPath: streamID.EncryptedObjectKey,
		Time:          time.Now(),
	}, console.RateLimitList)
	if err != nil {
		return nil, err
	}
	endpoint.usageTracking(keyInfo, req.Header, fmt.Sprintf("%T", req))

	cursor := req.CursorPosition
	if cursor == nil {
		cursor = &pb.SegmentPosition{}
	}

	id, err := uuid.FromBytes(streamID.StreamId)
	if err != nil {
		return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to parse stream id")
	}

	result, err := endpoint.metabase.ListStreamPositions(ctx, metabase.ListStreamPositions{
		ProjectID: keyInfo.ProjectID,
		StreamID:  id,
		Cursor: metabase.SegmentPosition{
			Part:  uint32(cursor.PartNumber),
			Index: uint32(cursor.Index),
		},
		Limit: int(req.Limit),
	})
	if err != nil {
		return nil, endpoint.ConvertMetabaseErr(err)
	}

	response, err := convertStreamListResults(result)
	if err != nil {
		return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to convert stream list")
	}
	response.EncryptionParameters = streamID.EncryptionParameters

	mon.Meter("req_list_segments").Mark(1)

	return response, nil
}

func convertStreamListResults(result metabase.ListStreamPositionsResult) (*pb.SegmentListResponse, error) {
	items := make([]*pb.SegmentListItem, len(result.Segments))
	for i, item := range result.Segments {
		items[i] = &pb.SegmentListItem{
			Position: &pb.SegmentPosition{
				PartNumber: int32(item.Position.Part),
				Index:      int32(item.Position.Index),
			},
			PlainSize:   int64(item.PlainSize),
			PlainOffset: item.PlainOffset,
		}
		if item.CreatedAt != nil {
			items[i].CreatedAt = *item.CreatedAt
		}
		items[i].EncryptedETag = item.EncryptedETag
		var err error
		items[i].EncryptedKeyNonce, err = storj.NonceFromBytes(item.EncryptedKeyNonce)
		if err != nil {
			return nil, err
		}
		items[i].EncryptedKey = item.EncryptedKey
	}
	return &pb.SegmentListResponse{
		Items: items,
		More:  result.More,
	}, nil
}

// DownloadSegment returns data necessary to download segment.
func (endpoint *Endpoint) DownloadSegment(ctx context.Context, req *pb.SegmentDownloadRequest) (resp *pb.SegmentDownloadResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if ctx.Err() != nil {
		return nil, rpcstatus.Error(rpcstatus.Canceled, "client has closed the connection")
	}

	endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())

	peer, trusted, err := endpoint.uplinkPeer(ctx)
	if err != nil {
		// N.B. jeff thinks this is a bad idea but jt convinced him
		return nil, err
	}

	streamID, err := endpoint.unmarshalSatStreamID(ctx, req.StreamId)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	keyInfo, err := endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:            macaroon.ActionRead,
		Bucket:        streamID.Bucket,
		EncryptedPath: streamID.EncryptedObjectKey,
		Time:          time.Now(),
	}, console.RateLimitGet)
	if err != nil {
		return nil, err
	}
	endpoint.usageTracking(keyInfo, req.Header, fmt.Sprintf("%T", req))

	if err := validateServerSideCopyFlag(req.ServerSideCopy, trusted); err != nil {
		return nil, err
	}

	if err := endpoint.checkDownloadLimits(ctx, keyInfo); err != nil {
		return nil, err
	}

	id, err := uuid.FromBytes(streamID.StreamId)
	if err != nil {
		return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to parse stream id")
	}

	var segment metabase.Segment
	if req.CursorPosition.PartNumber == 0 && req.CursorPosition.Index == -1 {
		if streamID.MultipartObject {
			return nil, rpcstatus.Error(rpcstatus.Unimplemented, "Used uplink version cannot download multipart objects.")
		}

		segment, err = endpoint.metabase.GetLatestObjectLastSegment(ctx, metabase.GetLatestObjectLastSegment{
			ObjectLocation: metabase.ObjectLocation{
				ProjectID:  keyInfo.ProjectID,
				BucketName: metabase.BucketName(streamID.Bucket),
				ObjectKey:  metabase.ObjectKey(streamID.EncryptedObjectKey),
			},
		})
	} else {
		segment, err = endpoint.metabase.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
			StreamID: id,
			Position: metabase.SegmentPosition{
				Part:  uint32(req.CursorPosition.PartNumber),
				Index: uint32(req.CursorPosition.Index),
			},
		})
	}
	if err != nil {
		return nil, endpoint.ConvertMetabaseErr(err)
	}

	// Update the current bandwidth cache value incrementing the SegmentSize.
	err = endpoint.projectUsage.UpdateProjectBandwidthUsage(ctx, keyInfoToLimits(keyInfo), int64(segment.EncryptedSize))
	if err != nil {
		// don't log errors if it was user cancellation
		if errors.Is(ctx.Err(), context.Canceled) {
			return nil, rpcstatus.Wrap(rpcstatus.Canceled, err)
		}

		// log it and continue. it's most likely our own fault that we couldn't
		// track it, and the only thing that will be affected is our per-project
		// bandwidth limits.
		endpoint.log.Error("Could not track the new project's bandwidth usage when downloading a segment",
			zap.Stringer("Project ID", keyInfo.ProjectID),
			zap.Error(err),
		)
	}

	encryptedKeyNonce, err := storj.NonceFromBytes(segment.EncryptedKeyNonce)
	if err != nil {
		return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to get encryption key nonce from metadata")
	}

	bucket := metabase.BucketLocation{ProjectID: keyInfo.ProjectID, BucketName: metabase.BucketName(streamID.Bucket)}

	if segment.Inline() {
		// skip egress tracking for server-side copy operation
		if !req.ServerSideCopy {
			if err := endpoint.orders.UpdateGetInlineOrder(ctx, bucket, int64(len(segment.InlineData))); err != nil {
				return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to update GET inline order")
			}
		}

		endpoint.versionCollector.collectTransferStats(req.Header.UserAgent, download, len(segment.InlineData))

		endpoint.log.Debug("Inline Segment Download", zap.Stringer("Project ID", keyInfo.ProjectID), zap.String("operation", "get"), zap.String("type", "inline"))
		mon.Meter("req_get_inline").Mark(1)

		return &pb.SegmentDownloadResponse{
			PlainOffset:         segment.PlainOffset,
			PlainSize:           int64(segment.PlainSize),
			SegmentSize:         int64(segment.EncryptedSize),
			EncryptedInlineData: segment.InlineData,

			EncryptedKeyNonce: encryptedKeyNonce,
			EncryptedKey:      segment.EncryptedKey,
			Position: &pb.SegmentPosition{
				PartNumber: int32(segment.Position.Part),
				Index:      int32(segment.Position.Index),
			},
		}, nil
	}

	if req.ServerSideCopy {
		// skip egress tracking for server-side copy operation, empty bucket location will
		// be skipped while orders settlement
		bucket = metabase.BucketLocation{}
	}

	// Remote segment
	limits, privateKey, err := endpoint.orders.CreateGetOrderLimits(ctx, peer, bucket, segment, req.GetDesiredNodes(), 0)
	if err != nil {
		if orders.ErrDownloadFailedNotEnoughPieces.Has(err) {
			endpoint.log.Error("Unable to create order limits.",
				zap.Stringer("Project ID", keyInfo.ProjectID),
				zap.Stringer("API Key ID", keyInfo.ID),
				zap.Error(err),
			)
		}
		return nil, endpoint.ConvertKnownErrWithMessage(err, "unable to create order limits")
	}

	endpoint.versionCollector.collectTransferStats(req.Header.UserAgent, download, int(segment.EncryptedSize))

	endpoint.log.Debug("Segment Download", zap.Stringer("Project ID", keyInfo.ProjectID), zap.String("operation", "get"), zap.String("type", "remote"))
	mon.Meter("req_get_remote").Mark(1)

	return &pb.SegmentDownloadResponse{
		AddressedLimits: limits,
		PrivateKey:      privateKey,
		PlainOffset:     segment.PlainOffset,
		PlainSize:       int64(segment.PlainSize),
		SegmentSize:     int64(segment.EncryptedSize),

		EncryptedKeyNonce: encryptedKeyNonce,
		EncryptedKey:      segment.EncryptedKey,
		RedundancyScheme: &pb.RedundancyScheme{
			Type:             pb.RedundancyScheme_SchemeType(segment.Redundancy.Algorithm),
			ErasureShareSize: segment.Redundancy.ShareSize,

			MinReq:           int32(segment.Redundancy.RequiredShares),
			RepairThreshold:  int32(segment.Redundancy.RepairShares),
			SuccessThreshold: int32(segment.Redundancy.OptimalShares),
			Total:            int32(segment.Redundancy.TotalShares),
		},
		Position: &pb.SegmentPosition{
			PartNumber: int32(segment.Position.Part),
			Index:      int32(segment.Position.Index),
		},
	}, nil
}

// DeletePart is a no-op.
//
// It was used to perform the deletion of a single part from satellite db and
// from storage nodes. We made this method noop because now we can overwrite
// segments for pending objects. It's returning no error to avoid failures with
// uplinks that still are using this method.
func (endpoint *Endpoint) DeletePart(ctx context.Context, req *pb.PartDeleteRequest) (resp *pb.PartDeleteResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	return &pb.PartDeleteResponse{}, nil
}
