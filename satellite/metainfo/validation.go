// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"bytes"
	"context"
	"regexp"
	"sync"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"storj.io/common/encryption"
	"storj.io/common/macaroon"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/pkg/auth"
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

func getAPIKey(ctx context.Context, header *pb.RequestHeader) (key *macaroon.APIKey, err error) {
	defer mon.Task()(&ctx)(&err)
	if header != nil {
		return macaroon.ParseRawAPIKey(header.ApiKey)
	}

	keyData, ok := auth.GetAPIKey(ctx)
	if !ok {
		return nil, errs.New("missing credentials")
	}

	return macaroon.ParseAPIKey(string(keyData))
}

// validateAuth validates things like API key, user permissions and rate limit and always returns valid rpc error.
func (endpoint *Endpoint) validateAuth(ctx context.Context, header *pb.RequestHeader, action macaroon.Action) (_ *console.APIKeyInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	key, err := getAPIKey(ctx, header)
	if err != nil {
		endpoint.log.Debug("invalid request", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, "Invalid API credentials")
	}

	keyInfo, err := endpoint.apiKeys.GetByHead(ctx, key.Head())
	if err != nil {
		endpoint.log.Debug("unauthorized request", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.PermissionDenied, "Unauthorized API credentials")
	}

	if err = endpoint.checkRate(ctx, keyInfo.ProjectID); err != nil {
		endpoint.log.Debug("rate check failed", zap.Error(err))
		return nil, err
	}

	// Revocations are currently handled by just deleting the key.
	err = key.Check(ctx, keyInfo.Secret, action, nil)
	if err != nil {
		endpoint.log.Debug("unauthorized request", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.PermissionDenied, "Unauthorized API credentials")
	}

	return keyInfo, nil
}

// getKeyInfo returns key info based on the header.
func (endpoint *Endpoint) getKeyInfo(ctx context.Context, header *pb.RequestHeader) (_ *console.APIKeyInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	key, err := getAPIKey(ctx, header)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, "Invalid API credentials")
	}

	keyInfo, err := endpoint.apiKeys.GetByHead(ctx, key.Head())
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.PermissionDenied, "Unauthorized API credentials")
	}

	return keyInfo, nil
}

func (endpoint *Endpoint) checkRate(ctx context.Context, projectID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)
	if !endpoint.config.RateLimiter.Enabled {
		return nil
	}
	limiter, err := endpoint.limiterCache.Get(projectID.String(), func() (interface{}, error) {
		limit := rate.Limit(endpoint.config.RateLimiter.Rate)

		project, err := endpoint.projects.Get(ctx, projectID)
		if err != nil {
			return false, err
		}
		if project.RateLimit != nil && *project.RateLimit > 0 {
			limit = rate.Limit(*project.RateLimit)
		}

		// initialize the limiter with limit and burst the same so that we don't limit how quickly calls
		// are made within the second.
		return rate.NewLimiter(limit, int(limit)), nil
	})
	if err != nil {
		return rpcstatus.Error(rpcstatus.Unavailable, err.Error())
	}

	if !limiter.(*rate.Limiter).Allow() {
		endpoint.log.Warn("too many requests for project",
			zap.Stringer("projectID", projectID),
			zap.Float64("limit", float64(limiter.(*rate.Limiter).Limit())))

		mon.Event("metainfo_rate_limit_exceeded") //locked

		return rpcstatus.Error(rpcstatus.ResourceExhausted, "Too Many Requests")
	}

	return nil
}

func (endpoint *Endpoint) validateCommitSegment(ctx context.Context, req *pb.SegmentCommitRequestOld) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = endpoint.validateBucket(ctx, req.Bucket)
	if err != nil {
		return err
	}

	err = endpoint.validatePointer(ctx, req.Pointer, req.OriginalLimits)
	if err != nil {
		return err
	}

	if len(req.OriginalLimits) > 0 {
		createRequest, found := endpoint.createRequests.Load(req.OriginalLimits[0].SerialNumber)

		switch {
		case !found:
			return Error.New("missing create request or request expired")
		case !createRequest.Expiration.Equal(req.Pointer.ExpirationDate):
			return Error.New("pointer expiration date does not match requested one")
		case !pb.Equal(createRequest.Redundancy, req.Pointer.Remote.Redundancy):
			return Error.New("pointer redundancy scheme date does not match requested one")
		}
	}

	return nil
}

func (endpoint *Endpoint) validateBucket(ctx context.Context, bucket []byte) (err error) {
	defer mon.Task()(&ctx)(&err)

	if len(bucket) == 0 {
		return Error.Wrap(storj.ErrNoBucket.New(""))
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

func (endpoint *Endpoint) validatePointer(ctx context.Context, pointer *pb.Pointer, originalLimits []*pb.OrderLimit) (err error) {
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

		remote := pointer.Remote

		if len(originalLimits) == 0 {
			return Error.New("no order limits")
		}
		if int32(len(originalLimits)) != remote.Redundancy.Total {
			return Error.New("invalid no order limit for piece")
		}

		maxAllowed, err := encryption.CalcEncryptedSize(endpoint.config.MaxSegmentSize.Int64(), storj.EncryptionParameters{
			CipherSuite: storj.EncAESGCM,
			BlockSize:   128, // intentionally low block size to allow maximum possible encryption overhead
		})
		if err != nil {
			return err
		}

		if pointer.SegmentSize > maxAllowed || pointer.SegmentSize < 0 {
			return Error.New("segment size %v is out of range, maximum allowed is %v", pointer.SegmentSize, maxAllowed)
		}

		pieceNums := make(map[int32]struct{})
		nodeIds := make(map[storj.NodeID]struct{})
		for _, piece := range remote.RemotePieces {
			if piece.PieceNum >= int32(len(originalLimits)) {
				return Error.New("invalid piece number")
			}

			limit := originalLimits[piece.PieceNum]

			if limit == nil {
				return Error.New("empty order limit for piece")
			}

			err := endpoint.orders.VerifyOrderLimitSignature(ctx, limit)
			if err != nil {
				return err
			}

			// expect that too much time has not passed between order limit creation and now
			if time.Since(limit.OrderCreation) > endpoint.config.MaxCommitInterval {
				return Error.New("Segment not committed before max commit interval of %f minutes.", endpoint.config.MaxCommitInterval.Minutes())
			}

			derivedPieceID := remote.RootPieceId.Derive(piece.NodeId, piece.PieceNum)
			if limit.PieceId.IsZero() || limit.PieceId != derivedPieceID {
				return Error.New("invalid order limit piece id")
			}
			if piece.NodeId != limit.StorageNodeId {
				return Error.New("piece NodeID != order limit NodeID")
			}

			if _, ok := pieceNums[piece.PieceNum]; ok {
				return Error.New("piece num %d is duplicated", piece.PieceNum)
			}

			if _, ok := nodeIds[piece.NodeId]; ok {
				return Error.New("node id %s for piece num %d is duplicated", piece.NodeId.String(), piece.PieceNum)
			}

			pieceNums[piece.PieceNum] = struct{}{}
			nodeIds[piece.NodeId] = struct{}{}
		}
	}

	return nil
}

func (endpoint *Endpoint) validateRedundancy(ctx context.Context, redundancy *pb.RedundancyScheme) (err error) {
	defer mon.Task()(&ctx)(&err)

	if endpoint.config.RS.Validate {
		if endpoint.config.RS.ErasureShareSize.Int32() != redundancy.ErasureShareSize ||
			endpoint.config.RS.MinTotalThreshold > int(redundancy.Total) ||
			endpoint.config.RS.MaxTotalThreshold < int(redundancy.Total) ||
			endpoint.config.RS.MinThreshold != int(redundancy.MinReq) ||
			endpoint.config.RS.RepairThreshold != int(redundancy.RepairThreshold) ||
			endpoint.config.RS.SuccessThreshold != int(redundancy.SuccessThreshold) {
			return Error.New("provided redundancy scheme parameters not allowed: want [%d, %d, %d, %d-%d, %d] got [%d, %d, %d, %d, %d]",
				endpoint.config.RS.MinThreshold,
				endpoint.config.RS.RepairThreshold,
				endpoint.config.RS.SuccessThreshold,
				endpoint.config.RS.MinTotalThreshold,
				endpoint.config.RS.MaxTotalThreshold,
				endpoint.config.RS.ErasureShareSize.Int32(),

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
