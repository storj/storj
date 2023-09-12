// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jtolio/eventkit"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"storj.io/common/encryption"
	"storj.io/common/lrucache"
	"storj.io/common/macaroon"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/attribution"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/internalpb"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metainfo/pointerverification"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/revocation"
)

const (
	satIDExpiration = 48 * time.Hour
)

var (
	mon = monkit.Package()
	evs = eventkit.Package()

	// Error general metainfo error.
	Error = errs.Class("metainfo")
	// ErrNodeAlreadyExists pointer already has a piece for a node err.
	ErrNodeAlreadyExists = errs.Class("metainfo: node already exists")
	// ErrBucketNotEmpty is returned when bucket is required to be empty for an operation.
	ErrBucketNotEmpty = errs.Class("bucket not empty")
)

// APIKeys is api keys store methods used by endpoint.
//
// architecture: Database
type APIKeys interface {
	GetByHead(ctx context.Context, head []byte) (*console.APIKeyInfo, error)
}

// Endpoint metainfo endpoint.
//
// architecture: Endpoint
type Endpoint struct {
	pb.DRPCMetainfoUnimplementedServer

	log                    *zap.Logger
	buckets                *buckets.Service
	metabase               *metabase.DB
	orders                 *orders.Service
	overlay                *overlay.Service
	attributions           attribution.DB
	pointerVerification    *pointerverification.Service
	projectUsage           *accounting.Service
	projectLimits          *accounting.ProjectLimitCache
	projects               console.Projects
	apiKeys                APIKeys
	satellite              signing.Signer
	limiterCache           *lrucache.ExpiringLRUOf[*rate.Limiter]
	singleObjectLimitCache *lrucache.ExpiringLRUOf[struct{}]
	encInlineSegmentSize   int64 // max inline segment size + encryption overhead
	revocations            revocation.DB
	defaultRS              *pb.RedundancyScheme
	config                 ExtendedConfig
	versionCollector       *versionCollector
}

// NewEndpoint creates new metainfo endpoint instance.
func NewEndpoint(log *zap.Logger, buckets *buckets.Service, metabaseDB *metabase.DB,
	orders *orders.Service, cache *overlay.Service, attributions attribution.DB, peerIdentities overlay.PeerIdentities,
	apiKeys APIKeys, projectUsage *accounting.Service, projectLimits *accounting.ProjectLimitCache, projects console.Projects,
	satellite signing.Signer, revocations revocation.DB, config Config) (*Endpoint, error) {
	// TODO do something with too many params

	extendedConfig, err := NewExtendedConfig(config)
	if err != nil {
		return nil, err
	}

	encInlineSegmentSize, err := encryption.CalcEncryptedSize(config.MaxInlineSegmentSize.Int64(), storj.EncryptionParameters{
		CipherSuite: storj.EncAESGCM,
		BlockSize:   128, // intentionally low block size to allow maximum possible encryption overhead
	})
	if err != nil {
		return nil, err
	}

	defaultRSScheme := &pb.RedundancyScheme{
		Type:             pb.RedundancyScheme_RS,
		MinReq:           int32(config.RS.Min),
		RepairThreshold:  int32(config.RS.Repair),
		SuccessThreshold: int32(config.RS.Success),
		Total:            int32(config.RS.Total),
		ErasureShareSize: config.RS.ErasureShareSize.Int32(),
	}

	return &Endpoint{
		log:                 log,
		buckets:             buckets,
		metabase:            metabaseDB,
		orders:              orders,
		overlay:             cache,
		attributions:        attributions,
		pointerVerification: pointerverification.NewService(peerIdentities),
		apiKeys:             apiKeys,
		projectUsage:        projectUsage,
		projectLimits:       projectLimits,
		projects:            projects,
		satellite:           satellite,
		limiterCache: lrucache.NewOf[*rate.Limiter](lrucache.Options{
			Capacity:   config.RateLimiter.CacheCapacity,
			Expiration: config.RateLimiter.CacheExpiration,
			Name:       "metainfo-ratelimit",
		}),
		singleObjectLimitCache: lrucache.NewOf[struct{}](lrucache.Options{
			Expiration: config.UploadLimiter.SingleObjectLimit,
			Capacity:   config.UploadLimiter.CacheCapacity,
		}),
		encInlineSegmentSize: encInlineSegmentSize,
		revocations:          revocations,
		defaultRS:            defaultRSScheme,
		config:               extendedConfig,
		versionCollector:     newVersionCollector(log),
	}, nil
}

// Close closes resources.
func (endpoint *Endpoint) Close() error { return nil }

// ProjectInfo returns allowed ProjectInfo for the provided API key.
func (endpoint *Endpoint) ProjectInfo(ctx context.Context, req *pb.ProjectInfoRequest) (_ *pb.ProjectInfoResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())

	keyInfo, err := endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:   macaroon.ActionProjectInfo,
		Time: time.Now(),
	})
	if err != nil {
		return nil, err
	}
	endpoint.usageTracking(keyInfo, req.Header, fmt.Sprintf("%T", req))

	salt, err := endpoint.projects.GetSalt(ctx, keyInfo.ProjectID)
	if err != nil {
		return nil, err
	}

	return &pb.ProjectInfoResponse{
		ProjectPublicId: keyInfo.ProjectPublicID.Bytes(),
		ProjectSalt:     salt,
	}, nil
}

// RevokeAPIKey handles requests to revoke an api key.
func (endpoint *Endpoint) RevokeAPIKey(ctx context.Context, req *pb.RevokeAPIKeyRequest) (resp *pb.RevokeAPIKeyResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())

	macToRevoke, err := macaroon.ParseMacaroon(req.GetApiKey())
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, "API key to revoke is not a macaroon")
	}
	keyInfo, err := endpoint.validateRevoke(ctx, req.Header, macToRevoke)
	if err != nil {
		return nil, err
	}
	endpoint.usageTracking(keyInfo, req.Header, fmt.Sprintf("%T", req))

	err = endpoint.revocations.Revoke(ctx, macToRevoke.Tail(), keyInfo.ID[:])
	if err != nil {
		endpoint.log.Error("Failed to revoke API key", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, "Failed to revoke API key")
	}

	return &pb.RevokeAPIKeyResponse{}, nil
}

func (endpoint *Endpoint) packStreamID(ctx context.Context, satStreamID *internalpb.StreamID) (streamID storj.StreamID, err error) {
	defer mon.Task()(&ctx)(&err)

	if satStreamID == nil {
		return nil, rpcstatus.Error(rpcstatus.Internal, "unable to create stream id")
	}

	if !satStreamID.ExpirationDate.IsZero() {
		// DB can only preserve microseconds precision and nano seconds will be cut.
		// To have stable StreamID/UploadID we need to always truncate it.
		satStreamID.ExpirationDate = satStreamID.ExpirationDate.Truncate(time.Microsecond)
	}

	signedStreamID, err := SignStreamID(ctx, endpoint.satellite, satStreamID)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	encodedStreamID, err := pb.Marshal(signedStreamID)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	streamID, err = storj.StreamIDFromBytes(encodedStreamID)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}
	return streamID, nil
}

func (endpoint *Endpoint) packSegmentID(ctx context.Context, satSegmentID *internalpb.SegmentID) (segmentID storj.SegmentID, err error) {
	defer mon.Task()(&ctx)(&err)

	if satSegmentID == nil {
		return nil, rpcstatus.Error(rpcstatus.Internal, "unable to create segment id")
	}

	signedSegmentID, err := SignSegmentID(ctx, endpoint.satellite, satSegmentID)
	if err != nil {
		return nil, err
	}

	encodedSegmentID, err := pb.Marshal(signedSegmentID)
	if err != nil {
		return nil, err
	}

	segmentID, err = storj.SegmentIDFromBytes(encodedSegmentID)
	if err != nil {
		return nil, err
	}
	return segmentID, nil
}

func (endpoint *Endpoint) unmarshalSatStreamID(ctx context.Context, streamID storj.StreamID) (_ *internalpb.StreamID, err error) {
	defer mon.Task()(&ctx)(&err)

	satStreamID := &internalpb.StreamID{}
	err = pb.Unmarshal(streamID, satStreamID)
	if err != nil {
		return nil, err
	}

	err = VerifyStreamID(ctx, endpoint.satellite, satStreamID)
	if err != nil {
		return nil, err
	}

	return satStreamID, nil
}

func (endpoint *Endpoint) unmarshalSatSegmentID(ctx context.Context, segmentID storj.SegmentID) (_ *internalpb.SegmentID, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(segmentID) == 0 {
		return nil, errs.New("segment ID missing")
	}

	satSegmentID := &internalpb.SegmentID{}
	err = pb.Unmarshal(segmentID, satSegmentID)
	if err != nil {
		return nil, err
	}
	if satSegmentID.StreamId == nil {
		return nil, errs.New("stream ID missing")
	}

	err = VerifySegmentID(ctx, endpoint.satellite, satSegmentID)
	if err != nil {
		return nil, err
	}

	if satSegmentID.CreationDate.Before(time.Now().Add(-satIDExpiration)) {
		return nil, errs.New("segment ID expired")
	}

	return satSegmentID, nil
}

// convertMetabaseErr converts domain errors from metabase to appropriate rpc statuses errors.
func (endpoint *Endpoint) convertMetabaseErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, context.Canceled):
		return rpcstatus.Error(rpcstatus.Canceled, "context canceled")
	case errors.Is(err, context.DeadlineExceeded):
		return rpcstatus.Error(rpcstatus.DeadlineExceeded, "context deadline exceeded")
	case rpcstatus.Code(err) != rpcstatus.Unknown:
		// it's already RPC error
		return err
	case metabase.ErrObjectNotFound.Has(err):
		message := strings.TrimPrefix(err.Error(), string(metabase.ErrObjectNotFound))
		message = strings.TrimPrefix(message, ": ")
		// uplink expects a message that starts with the specified prefix
		return rpcstatus.Error(rpcstatus.NotFound, "object not found: "+message)
	case metabase.ErrSegmentNotFound.Has(err):
		message := strings.TrimPrefix(err.Error(), string(metabase.ErrSegmentNotFound))
		message = strings.TrimPrefix(message, ": ")
		// uplink expects a message that starts with the specified prefix
		return rpcstatus.Error(rpcstatus.NotFound, "segment not found: "+message)
	case metabase.ErrInvalidRequest.Has(err):
		return rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	case metabase.ErrObjectAlreadyExists.Has(err):
		return rpcstatus.Error(rpcstatus.AlreadyExists, err.Error())
	case metabase.ErrPendingObjectMissing.Has(err):
		return rpcstatus.Error(rpcstatus.NotFound, err.Error())
	case metabase.ErrPermissionDenied.Has(err):
		return rpcstatus.Error(rpcstatus.PermissionDenied, err.Error())
	default:
		endpoint.log.Error("internal", zap.Error(err))
		return rpcstatus.Error(rpcstatus.Internal, "internal error")
	}
}

func (endpoint *Endpoint) usageTracking(keyInfo *console.APIKeyInfo, header *pb.RequestHeader, name string, tags ...eventkit.Tag) {
	evs.Event("usage", append([]eventkit.Tag{
		eventkit.Bytes("project-public-id", keyInfo.ProjectPublicID[:]),
		eventkit.String("user-agent", string(header.UserAgent)),
		eventkit.String("request", name),
	}, tags...)...)
}
