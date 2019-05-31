// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"bytes"
	"context"

	"github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"storj.io/storj/pkg/auth"
	"storj.io/storj/pkg/macaroon"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/satellite/console"
)

func (endpoint *Endpoint) validateAuth(ctx context.Context, action macaroon.Action) (*console.APIKeyInfo, error) {
	keyData, ok := auth.GetAPIKey(ctx)
	if !ok {
		endpoint.log.Error("unauthorized request", zap.Error(status.Errorf(codes.Unauthenticated, "Invalid API credential")))
		return nil, status.Errorf(codes.Unauthenticated, "Invalid API credential")
	}

	key, err := macaroon.ParseAPIKey(string(keyData))
	if err != nil {
		endpoint.log.Error("unauthorized request", zap.Error(status.Errorf(codes.Unauthenticated, "Invalid API credential")))
		return nil, status.Errorf(codes.Unauthenticated, "Invalid API credential")
	}

	keyInfo, err := endpoint.apiKeys.GetByHead(ctx, key.Head())
	if err != nil {
		endpoint.log.Error("unauthorized request", zap.Error(status.Errorf(codes.Unauthenticated, err.Error())))
		return nil, status.Errorf(codes.Unauthenticated, "Invalid API credential")
	}

	// Revocations are currently handled by just deleting the key.
	err = key.Check(keyInfo.Secret, action, nil)
	if err != nil {
		endpoint.log.Error("unauthorized request", zap.Error(status.Errorf(codes.Unauthenticated, err.Error())))
		return nil, status.Errorf(codes.Unauthenticated, "Invalid API credential")
	}

	return keyInfo, nil
}

func (endpoint *Endpoint) validateCreateSegment(req *pb.SegmentWriteRequest) error {
	err := endpoint.validateBucket(req.Bucket)
	if err != nil {
		return err
	}

	err = endpoint.validateRedundancy(req.Redundancy)
	if err != nil {
		return err
	}

	return nil
}

func (endpoint *Endpoint) validateCommitSegment(req *pb.SegmentCommitRequest) error {
	err := endpoint.validateBucket(req.Bucket)
	if err != nil {
		return err
	}

	err = endpoint.validatePointer(req.Pointer)
	if err != nil {
		return err
	}

	if req.Pointer.Type == pb.Pointer_REMOTE {
		remote := req.Pointer.Remote

		if len(req.OriginalLimits) == 0 {
			return Error.New("no order limits")
		}
		if int32(len(req.OriginalLimits)) != remote.Redundancy.Total {
			return Error.New("invalid no order limit for piece")
		}

		for _, piece := range remote.RemotePieces {
			limit := req.OriginalLimits[piece.PieceNum]

			err := endpoint.orders.VerifyOrderLimitSignature(limit)
			if err != nil {
				return err
			}

			if limit == nil {
				return Error.New("invalid no order limit for piece")
			}
			derivedPieceID := remote.RootPieceId.Derive(piece.NodeId)
			if limit.PieceId.IsZero() || limit.PieceId != derivedPieceID {
				return Error.New("invalid order limit piece id")
			}
			if bytes.Compare(piece.NodeId.Bytes(), limit.StorageNodeId.Bytes()) != 0 {
				return Error.New("piece NodeID != order limit NodeID")
			}
		}
	}

	if len(req.OriginalLimits) > 0 {
		createRequest, found := endpoint.createRequests.Request(req.OriginalLimits[0].SerialNumber)

		switch {
		case !found:
			return Error.New("no create request for remote segment")
		case !proto.Equal(createRequest.Expiration, req.Pointer.ExpirationDate):
			return Error.New("pointer expiration date does not match requested one")
		case !proto.Equal(createRequest.Redundancy, req.Pointer.Remote.Redundancy):
			return Error.New("pointer redundancy scheme date does not match requested one")
		}
	}

	return nil
}

func (endpoint *Endpoint) validateBucket(bucket []byte) error {
	if len(bucket) == 0 {
		return errs.New("bucket not specified")
	}
	if bytes.ContainsAny(bucket, "/") {
		return errs.New("bucket should not contain slash")
	}
	return nil
}

func (endpoint *Endpoint) validatePointer(pointer *pb.Pointer) error {
	if pointer == nil {
		return Error.New("no pointer specified")
	}

	if pointer.Type == pb.Pointer_INLINE && pointer.Remote != nil {
		return Error.New("pointer type is INLINE but remote segment is set")
	}

	// TODO does it all?
	if pointer.Type == pb.Pointer_REMOTE {
		if pointer.Remote == nil {
			return Error.New("no remote segment specified")
		}
		if pointer.Remote.RemotePieces == nil {
			return Error.New("no remote segment pieces specified")
		}
		if pointer.Remote.Redundancy == nil {
			return Error.New("no redundancy scheme specified")
		}
	}
	return nil
}

func (endpoint *Endpoint) validateRedundancy(redundancy *pb.RedundancyScheme) error {
	// TODO more validation, use validation from eestream.NewRedundancyStrategy
	if redundancy.ErasureShareSize <= 0 {
		return Error.New("erasure share size cannot be less than 0")
	}
	return nil
}
