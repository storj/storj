// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"bytes"
	"context"
	"regexp"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"storj.io/storj/pkg/auth"
	"storj.io/storj/pkg/macaroon"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/console"
)

const (
	requestTTL = time.Hour * 4
)

var (
	ipRegexp = regexp.MustCompile(`^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$`)
)

// TTLItem keeps association between serial number and ttl
type TTLItem struct {
	serialNumber storj.SerialNumber
	ttl          time.Time
}

type createRequest struct {
	Expiration time.Time
	Redundancy *pb.RedundancyScheme

	ttl time.Time
}

type createRequests struct {
	mu sync.RWMutex
	// orders limit serial number used because with CreateSegment we don't have path yet
	entries map[storj.SerialNumber]*createRequest

	muTTL      sync.Mutex
	entriesTTL []*TTLItem
}

func newCreateRequests() *createRequests {
	return &createRequests{
		entries:    make(map[storj.SerialNumber]*createRequest),
		entriesTTL: make([]*TTLItem, 0),
	}
}

func (requests *createRequests) Put(serialNumber storj.SerialNumber, createRequest *createRequest) {
	ttl := time.Now().Add(requestTTL)

	go func() {
		requests.muTTL.Lock()
		requests.entriesTTL = append(requests.entriesTTL, &TTLItem{
			serialNumber: serialNumber,
			ttl:          ttl,
		})
		requests.muTTL.Unlock()
	}()

	createRequest.ttl = ttl
	requests.mu.Lock()
	requests.entries[serialNumber] = createRequest
	requests.mu.Unlock()

	go requests.cleanup()
}

func (requests *createRequests) Load(serialNumber storj.SerialNumber) (*createRequest, bool) {
	requests.mu.RLock()
	request, found := requests.entries[serialNumber]
	if request != nil && request.ttl.Before(time.Now()) {
		request = nil
		found = false
	}
	requests.mu.RUnlock()

	return request, found
}

func (requests *createRequests) Remove(serialNumber storj.SerialNumber) {
	requests.mu.Lock()
	delete(requests.entries, serialNumber)
	requests.mu.Unlock()
}

func (requests *createRequests) cleanup() {
	requests.muTTL.Lock()
	now := time.Now()
	remove := make([]storj.SerialNumber, 0)
	newStart := 0
	for i, item := range requests.entriesTTL {
		if item.ttl.Before(now) {
			remove = append(remove, item.serialNumber)
			newStart = i + 1
		} else {
			break
		}
	}
	requests.entriesTTL = requests.entriesTTL[newStart:]
	requests.muTTL.Unlock()

	for _, serialNumber := range remove {
		requests.Remove(serialNumber)
	}
}

func (endpoint *Endpoint) validateAuth(ctx context.Context, action macaroon.Action) (_ *console.APIKeyInfo, err error) {
	defer mon.Task()(&ctx)(&err)
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
	err = key.Check(ctx, keyInfo.Secret, action, nil)
	if err != nil {
		endpoint.log.Error("unauthorized request", zap.Error(status.Errorf(codes.Unauthenticated, err.Error())))
		return nil, status.Errorf(codes.Unauthenticated, "Invalid API credential")
	}

	return keyInfo, nil
}

func (endpoint *Endpoint) validateCreateSegment(ctx context.Context, req *pb.SegmentWriteRequestOld) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = endpoint.validateBucket(ctx, req.Bucket)
	if err != nil {
		return err
	}

	err = endpoint.validateRedundancy(ctx, req.Redundancy)
	if err != nil {
		return err
	}

	return nil
}

func (endpoint *Endpoint) validateCommitSegment(ctx context.Context, req *pb.SegmentCommitRequestOld) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = endpoint.validateBucket(ctx, req.Bucket)
	if err != nil {
		return err
	}

	err = endpoint.validatePointer(ctx, req.Pointer)
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

		if req.Pointer.SegmentSize > endpoint.rsConfig.MaxSegmentSize.Int64() || req.Pointer.SegmentSize < 0 {
			return Error.New("segment size %v is out of range, maximum is %v", req.Pointer.SegmentSize, endpoint.rsConfig.MaxSegmentSize)
		}

		for _, piece := range remote.RemotePieces {
			limit := req.OriginalLimits[piece.PieceNum]

			err := endpoint.orders.VerifyOrderLimitSignature(ctx, limit)
			if err != nil {
				return err
			}

			if limit == nil {
				return Error.New("empty order limit for piece")
			}
			derivedPieceID := remote.RootPieceId.Derive(piece.NodeId, piece.PieceNum)
			if limit.PieceId.IsZero() || limit.PieceId != derivedPieceID {
				return Error.New("invalid order limit piece id")
			}
			if bytes.Compare(piece.NodeId.Bytes(), limit.StorageNodeId.Bytes()) != 0 {
				return Error.New("piece NodeID != order limit NodeID")
			}
		}
	}

	if len(req.OriginalLimits) > 0 {
		createRequest, found := endpoint.createRequests.Load(req.OriginalLimits[0].SerialNumber)

		switch {
		case !found:
			return Error.New("missing create request or request expired")
		case !createRequest.Expiration.Equal(req.Pointer.ExpirationDate):
			return Error.New("pointer expiration date does not match requested one")
		case !proto.Equal(createRequest.Redundancy, req.Pointer.Remote.Redundancy):
			return Error.New("pointer redundancy scheme date does not match requested one")
		}
	}

	return nil
}

func (endpoint *Endpoint) validateBucket(ctx context.Context, bucket []byte) (err error) {
	defer mon.Task()(&ctx)(&err)

	if len(bucket) == 0 {
		return Error.New("bucket not specified")
	}

	if len(bucket) < 3 || len(bucket) > 63 {
		return Error.New("bucket name must be at least 3 and no more than 63 characters long")
	}

	// Regexp not used because benchmark shows it will be slower for valid bucket names
	// https://gist.github.com/mniewrzal/49de3af95f36e63e88fac24f565e444c
	labels := bytes.Split(bucket, []byte("."))
	for _, label := range labels {
		err = validateBucketLabel(label)
		if err != nil {
			return err
		}
	}

	if ipRegexp.MatchString(string(bucket)) {
		return Error.New("bucket name cannot be formatted as an IP address")
	}

	return nil
}

func validateBucketLabel(label []byte) error {
	if len(label) == 0 {
		return Error.New("bucket label cannot be empty")
	}

	if !isLowerLetter(label[0]) && !isDigit(label[0]) {
		return Error.New("bucket label must start with a lowercase letter or number")
	}

	if label[0] == '-' || label[len(label)-1] == '-' {
		return Error.New("bucket label cannot start or end with a hyphen")
	}

	for i := 1; i < len(label)-1; i++ {
		if !isLowerLetter(label[i]) && !isDigit(label[i]) && (label[i] != '-') && (label[i] != '.') {
			return Error.New("bucket name must contain only lowercase letters, numbers or hyphens")
		}
	}

	return nil
}

func isLowerLetter(r byte) bool {
	return r >= 'a' && r <= 'z'
}

func isDigit(r byte) bool {
	return r >= '0' && r <= '9'
}

func (endpoint *Endpoint) validatePointer(ctx context.Context, pointer *pb.Pointer) (err error) {
	defer mon.Task()(&ctx)(&err)

	if pointer == nil {
		return Error.New("no pointer specified")
	}

	if pointer.Type == pb.Pointer_INLINE && pointer.Remote != nil {
		return Error.New("pointer type is INLINE but remote segment is set")
	}

	if pointer.Type == pb.Pointer_REMOTE {
		switch {
		case pointer.Remote == nil:
			return Error.New("no remote segment specified")
		case pointer.Remote.RemotePieces == nil:
			return Error.New("no remote segment pieces specified")
		case pointer.Remote.Redundancy == nil:
			return Error.New("no redundancy scheme specified")
		}
	}
	return nil
}

func (endpoint *Endpoint) validateRedundancy(ctx context.Context, redundancy *pb.RedundancyScheme) (err error) {
	defer mon.Task()(&ctx)(&err)

	if endpoint.rsConfig.Validate == true {
		if endpoint.rsConfig.ErasureShareSize.Int32() != redundancy.ErasureShareSize ||
			endpoint.rsConfig.MaxThreshold != int(redundancy.Total) ||
			endpoint.rsConfig.MinThreshold != int(redundancy.MinReq) ||
			endpoint.rsConfig.RepairThreshold != int(redundancy.RepairThreshold) ||
			endpoint.rsConfig.SuccessThreshold != int(redundancy.SuccessThreshold) {
			return Error.New("provided redundancy scheme parameters not allowed: want [%d, %d, %d, %d, %d] got [%d, %d, %d, %d, %d]",
				endpoint.rsConfig.MinThreshold,
				endpoint.rsConfig.RepairThreshold,
				endpoint.rsConfig.SuccessThreshold,
				endpoint.rsConfig.MaxThreshold,
				endpoint.rsConfig.ErasureShareSize.Int32(),

				redundancy.MinReq,
				redundancy.RepairThreshold,
				redundancy.SuccessThreshold,
				redundancy.Total,
				redundancy.ErasureShareSize,
			)
		}
	}

	return nil
}

func (endpoint *Endpoint) validatePieceHash(ctx context.Context, piece *pb.RemotePiece, limits []*pb.OrderLimit) (err error) {
	defer mon.Task()(&ctx)(&err)

	if piece.Hash == nil {
		return errs.New("no piece hash, removing from pointer %v (%v)", piece.NodeId, piece.PieceNum)
	}

	timestamp := piece.Hash.Timestamp
	if timestamp.Before(time.Now().Add(-pieceHashExpiration)) {
		return errs.New("piece hash timestamp is too old (%v), removing from pointer %v (num: %v)", timestamp, piece.NodeId, piece.PieceNum)
	}

	limit := limits[piece.PieceNum]
	if limit != nil {
		switch {
		case limit.PieceId != piece.Hash.PieceId:
			return errs.New("piece hash pieceID doesn't match limit pieceID, removing from pointer (%v != %v)", piece.Hash.PieceId, limit.PieceId)
		case limit.Limit < piece.Hash.PieceSize:
			return errs.New("piece hash PieceSize is larger than order limit, removing from pointer (%v > %v)", piece.Hash.PieceSize, limit.Limit)
		}
	}
	return nil
}
