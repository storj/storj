// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"storj.io/common/errs2"
	"storj.io/common/macaroon"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/internalpb"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/uplink/private/eestream"
)

func calculateSpaceUsed(segmentSize int64, numberOfPieces int, rs storj.RedundancyScheme) (totalStored int64) {
	pieceSize := segmentSize / int64(rs.RequiredShares)
	return pieceSize * int64(numberOfPieces)
}

// BeginSegment begins segment uploading.
func (endpoint *Endpoint) BeginSegment(ctx context.Context, req *pb.SegmentBeginRequest) (resp *pb.SegmentBeginResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	err = endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())
	if err != nil {
		endpoint.log.Warn("unable to collect uplink version", zap.Error(err))
	}

	streamID, err := endpoint.unmarshalSatStreamID(ctx, req.StreamId)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	keyInfo, err := endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:            macaroon.ActionWrite,
		Bucket:        streamID.Bucket,
		EncryptedPath: streamID.EncryptedObjectKey,
		Time:          time.Now(),
	})
	if err != nil {
		return nil, err
	}

	// no need to validate streamID fields because it was validated during BeginObject

	if req.Position.Index < 0 {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, "segment index must be greater then 0")
	}

	if err := endpoint.checkUploadLimits(ctx, keyInfo.ProjectID); err != nil {
		return nil, err
	}

	redundancy, err := eestream.NewRedundancyStrategyFromProto(endpoint.defaultRS)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	maxPieceSize := eestream.CalcPieceSize(req.MaxOrderLimit, redundancy)

	request := overlay.FindStorageNodesRequest{
		RequestedCount: redundancy.TotalCount(),
		Placement:      storj.PlacementConstraint(streamID.Placement),
	}
	nodes, err := endpoint.overlay.FindStorageNodesForUpload(ctx, request)
	if err != nil {
		endpoint.log.Error("internal", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	bucket := metabase.BucketLocation{ProjectID: keyInfo.ProjectID, BucketName: string(streamID.Bucket)}
	rootPieceID, addressedLimits, piecePrivateKey, err := endpoint.orders.CreatePutOrderLimits(ctx, bucket, nodes, streamID.ExpirationDate, maxPieceSize)
	if err != nil {
		endpoint.log.Error("internal", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	id, err := uuid.FromBytes(streamID.StreamId)
	if err != nil {
		endpoint.log.Error("internal", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	pieces := metabase.Pieces{}
	for i, limit := range addressedLimits {
		pieces = append(pieces, metabase.Piece{
			Number:      uint16(i),
			StorageNode: limit.Limit.StorageNodeId,
		})
	}
	err = endpoint.metabase.BeginSegment(ctx, metabase.BeginSegment{
		ObjectStream: metabase.ObjectStream{
			ProjectID:  keyInfo.ProjectID,
			BucketName: string(streamID.Bucket),
			ObjectKey:  metabase.ObjectKey(streamID.EncryptedObjectKey),
			StreamID:   id,
			Version:    1,
		},
		Position: metabase.SegmentPosition{
			Part:  uint32(req.Position.PartNumber),
			Index: uint32(req.Position.Index),
		},
		RootPieceID: rootPieceID,
		Pieces:      pieces,
	})
	if err != nil {
		return nil, endpoint.convertMetabaseErr(err)
	}

	segmentID, err := endpoint.packSegmentID(ctx, &internalpb.SegmentID{
		StreamId:            streamID,
		PartNumber:          req.Position.PartNumber,
		Index:               req.Position.Index,
		OriginalOrderLimits: addressedLimits,
		RootPieceId:         rootPieceID,
		CreationDate:        time.Now(),
	})

	endpoint.log.Info("Segment Upload", zap.Stringer("Project ID", keyInfo.ProjectID), zap.String("operation", "put"), zap.String("type", "remote"))
	mon.Meter("req_put_remote").Mark(1)

	return &pb.SegmentBeginResponse{
		SegmentId:        segmentID,
		AddressedLimits:  addressedLimits,
		PrivateKey:       piecePrivateKey,
		RedundancyScheme: endpoint.defaultRS,
	}, nil
}

// CommitSegment commits segment after uploading.
func (endpoint *Endpoint) CommitSegment(ctx context.Context, req *pb.SegmentCommitRequest) (resp *pb.SegmentCommitResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	err = endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())
	if err != nil {
		endpoint.log.Warn("unable to collect uplink version", zap.Error(err))
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
	})
	if err != nil {
		return nil, err
	}

	// cheap basic verification
	if numResults := len(req.UploadResult); numResults < int(endpoint.defaultRS.GetSuccessThreshold()) {
		endpoint.log.Debug("the results of uploaded pieces for the segment is below the redundancy optimal threshold",
			zap.Int("upload pieces results", numResults),
			zap.Int32("redundancy optimal threshold", endpoint.defaultRS.GetSuccessThreshold()),
			zap.Stringer("Segment ID", req.SegmentId),
		)
		return nil, rpcstatus.Errorf(rpcstatus.InvalidArgument,
			"the number of results of uploaded pieces (%d) is below the optimal threshold (%d)",
			numResults, endpoint.defaultRS.GetSuccessThreshold(),
		)
	}

	rs := storj.RedundancyScheme{
		Algorithm:      storj.RedundancyAlgorithm(endpoint.defaultRS.Type),
		RequiredShares: int16(endpoint.defaultRS.MinReq),
		RepairShares:   int16(endpoint.defaultRS.RepairThreshold),
		OptimalShares:  int16(endpoint.defaultRS.SuccessThreshold),
		TotalShares:    int16(endpoint.defaultRS.Total),
		ShareSize:      endpoint.defaultRS.ErasureShareSize,
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
	validPieces, invalidPieces, err := endpoint.pointerVerification.SelectValidPieces(ctx, req.UploadResult, originalLimits)
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
			errMsg = fmt.Sprintf("%s. Invalid Pieces:", errMsg)
			for _, p := range invalidPieces {
				errMsg = fmt.Sprintf("%s\nNodeID: %v, PieceNum: %d, Reason: %s",
					errMsg, p.NodeID, p.PieceNum, p.Reason,
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
		endpoint.log.Error("internal", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	var expiresAt *time.Time
	if !streamID.ExpirationDate.IsZero() {
		expiresAt = &streamID.ExpirationDate
	}

	mbCommitSegment := metabase.CommitSegment{
		ObjectStream: metabase.ObjectStream{
			ProjectID:  keyInfo.ProjectID,
			BucketName: string(streamID.Bucket),
			ObjectKey:  metabase.ObjectKey(streamID.EncryptedObjectKey),
			StreamID:   id,
			Version:    1,
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
	}

	err = endpoint.validateRemoteSegment(ctx, mbCommitSegment, originalLimits)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	if err := endpoint.checkUploadLimits(ctx, keyInfo.ProjectID); err != nil {
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
		return nil, endpoint.convertMetabaseErr(err)
	}

	if err := endpoint.updateUploadLimits(ctx, keyInfo.ProjectID, segmentSize); err != nil {
		return nil, err
	}

	return &pb.SegmentCommitResponse{
		SuccessfulPieces: int32(len(pieces)),
	}, nil
}

// MakeInlineSegment makes inline segment on satellite.
func (endpoint *Endpoint) MakeInlineSegment(ctx context.Context, req *pb.SegmentMakeInlineRequest) (resp *pb.SegmentMakeInlineResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	err = endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())
	if err != nil {
		endpoint.log.Warn("unable to collect uplink version", zap.Error(err))
	}

	streamID, err := endpoint.unmarshalSatStreamID(ctx, req.StreamId)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	keyInfo, err := endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:            macaroon.ActionWrite,
		Bucket:        streamID.Bucket,
		EncryptedPath: streamID.EncryptedObjectKey,
		Time:          time.Now(),
	})
	if err != nil {
		return nil, err
	}

	if req.Position.Index < 0 {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, "segment index must be greater then 0")
	}

	inlineUsed := int64(len(req.EncryptedInlineData))
	if inlineUsed > endpoint.encInlineSegmentSize {
		return nil, rpcstatus.Errorf(rpcstatus.InvalidArgument, "inline segment size cannot be larger than %s", endpoint.config.MaxInlineSegmentSize)
	}

	id, err := uuid.FromBytes(streamID.StreamId)
	if err != nil {
		endpoint.log.Error("internal", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	if err := endpoint.checkUploadLimits(ctx, keyInfo.ProjectID); err != nil {
		return nil, err
	}

	var expiresAt *time.Time
	if !streamID.ExpirationDate.IsZero() {
		expiresAt = &streamID.ExpirationDate
	}

	err = endpoint.metabase.CommitInlineSegment(ctx, metabase.CommitInlineSegment{
		ObjectStream: metabase.ObjectStream{
			ProjectID:  keyInfo.ProjectID,
			BucketName: string(streamID.Bucket),
			ObjectKey:  metabase.ObjectKey(streamID.EncryptedObjectKey),
			StreamID:   id,
			Version:    1,
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
	})
	if err != nil {
		return nil, endpoint.convertMetabaseErr(err)
	}

	bucket := metabase.BucketLocation{ProjectID: keyInfo.ProjectID, BucketName: string(streamID.Bucket)}
	err = endpoint.orders.UpdatePutInlineOrder(ctx, bucket, inlineUsed)
	if err != nil {
		endpoint.log.Error("internal", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	if err := endpoint.updateUploadLimits(ctx, keyInfo.ProjectID, inlineUsed); err != nil {
		return nil, err
	}

	endpoint.log.Info("Inline Segment Upload", zap.Stringer("Project ID", keyInfo.ProjectID), zap.String("operation", "put"), zap.String("type", "inline"))
	mon.Meter("req_put_inline").Mark(1)

	return &pb.SegmentMakeInlineResponse{}, nil
}

// ListSegments list object segments.
func (endpoint *Endpoint) ListSegments(ctx context.Context, req *pb.SegmentListRequest) (resp *pb.SegmentListResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	streamID, err := endpoint.unmarshalSatStreamID(ctx, req.StreamId)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	_, err = endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:            macaroon.ActionRead,
		Bucket:        streamID.Bucket,
		EncryptedPath: streamID.EncryptedObjectKey,
		Time:          time.Now(),
	})
	if err != nil {
		return nil, err
	}

	cursor := req.CursorPosition
	if cursor == nil {
		cursor = &pb.SegmentPosition{}
	}

	id, err := uuid.FromBytes(streamID.StreamId)
	if err != nil {
		endpoint.log.Error("internal", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	result, err := endpoint.metabase.ListStreamPositions(ctx, metabase.ListStreamPositions{
		StreamID: id,
		Cursor: metabase.SegmentPosition{
			Part:  uint32(cursor.PartNumber),
			Index: uint32(cursor.Index),
		},
		Limit: int(req.Limit),
	})
	if err != nil {
		return nil, endpoint.convertMetabaseErr(err)
	}

	response, err := convertStreamListResults(result)
	if err != nil {
		endpoint.log.Error("unable to convert stream list", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}
	response.EncryptionParameters = streamID.EncryptionParameters
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

	err = endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())
	if err != nil {
		endpoint.log.Warn("unable to collect uplink version", zap.Error(err))
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
	})
	if err != nil {
		return nil, err
	}

	bucket := metabase.BucketLocation{ProjectID: keyInfo.ProjectID, BucketName: string(streamID.Bucket)}

	if exceeded, limit, err := endpoint.projectUsage.ExceedsBandwidthUsage(ctx, keyInfo.ProjectID); err != nil {
		if errs2.IsCanceled(err) {
			return nil, rpcstatus.Wrap(rpcstatus.Canceled, err)
		}

		endpoint.log.Error(
			"Retrieving project bandwidth total failed; bandwidth limit won't be enforced",
			zap.Stringer("Project ID", keyInfo.ProjectID),
			zap.Error(err),
		)
	} else if exceeded {
		endpoint.log.Warn("Monthly bandwidth limit exceeded",
			zap.Stringer("Limit", limit),
			zap.Stringer("Project ID", keyInfo.ProjectID),
		)
		return nil, rpcstatus.Error(rpcstatus.ResourceExhausted, "Exceeded Usage Limit")
	}

	id, err := uuid.FromBytes(streamID.StreamId)
	if err != nil {
		endpoint.log.Error("internal", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	var segment metabase.Segment
	if req.CursorPosition.PartNumber == 0 && req.CursorPosition.Index == -1 {
		if streamID.MultipartObject {
			return nil, rpcstatus.Error(rpcstatus.Unimplemented, "Used uplink version cannot download multipart objects.")
		}

		segment, err = endpoint.metabase.GetLatestObjectLastSegment(ctx, metabase.GetLatestObjectLastSegment{
			ObjectLocation: metabase.ObjectLocation{
				ProjectID:  keyInfo.ProjectID,
				BucketName: string(streamID.Bucket),
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
		return nil, endpoint.convertMetabaseErr(err)
	}

	// Update the current bandwidth cache value incrementing the SegmentSize.
	err = endpoint.projectUsage.UpdateProjectBandwidthUsage(ctx, keyInfo.ProjectID, int64(segment.EncryptedSize))
	if err != nil {
		if errs2.IsCanceled(err) {
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
		endpoint.log.Error("unable to get encryption key nonce from metadata", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	if segment.Inline() {
		err := endpoint.orders.UpdateGetInlineOrder(ctx, bucket, int64(len(segment.InlineData)))
		if err != nil {
			endpoint.log.Error("internal", zap.Error(err))
			return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
		}
		endpoint.log.Info("Inline Segment Download", zap.Stringer("Project ID", keyInfo.ProjectID), zap.String("operation", "get"), zap.String("type", "inline"))
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

	// Remote segment
	limits, privateKey, err := endpoint.orders.CreateGetOrderLimits(ctx, bucket, segment, 0)
	if err != nil {
		if orders.ErrDownloadFailedNotEnoughPieces.Has(err) {
			endpoint.log.Error("Unable to create order limits.",
				zap.Stringer("Project ID", keyInfo.ProjectID),
				zap.Stringer("API Key ID", keyInfo.ID),
				zap.Error(err),
			)
		}
		endpoint.log.Error("internal", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	endpoint.log.Info("Segment Download", zap.Stringer("Project ID", keyInfo.ProjectID), zap.String("operation", "get"), zap.String("type", "remote"))
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
