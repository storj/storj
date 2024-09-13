// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"storj.io/common/encryption"
	"storj.io/common/macaroon"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/eventkit"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/attribution"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/internalpb"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metainfo/pointerverification"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/revocation"
	"storj.io/storj/shared/lrucache"
)

const (
	satIDExpiration    = 48 * time.Hour
	objectLockedErrMsg = "object is protected by Object Lock settings"
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
	projects               console.Projects
	projectMembers         console.ProjectMembers
	users                  console.Users
	apiKeys                APIKeys
	satellite              signing.Signer
	limiterCache           *lrucache.ExpiringLRUOf[*rate.Limiter]
	singleObjectLimitCache *lrucache.ExpiringLRUOf[struct{}]
	userInfoCache          *lrucache.ExpiringLRUOf[*console.UserInfo]
	encInlineSegmentSize   int64 // max inline segment size + encryption overhead
	revocations            revocation.DB
	config                 ExtendedConfig
	versionCollector       *versionCollector
	zstdDecoder            *zstd.Decoder
	zstdEncoder            *zstd.Encoder
	successTrackers        *SuccessTrackers
	placement              nodeselection.PlacementDefinitions
}

// NewEndpoint creates new metainfo endpoint instance.
func NewEndpoint(log *zap.Logger, buckets *buckets.Service, metabaseDB *metabase.DB,
	orders *orders.Service, cache *overlay.Service, attributions attribution.DB, peerIdentities overlay.PeerIdentities,
	apiKeys APIKeys, projectUsage *accounting.Service, projects console.Projects, projectMembers console.ProjectMembers, users console.Users,
	satellite signing.Signer, revocations revocation.DB, successTrackers *SuccessTrackers, config Config, placement nodeselection.PlacementDefinitions) (*Endpoint, error) {

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

	decoder, err := zstd.NewReader(nil,
		zstd.WithDecoderMaxMemory(64<<20),
	)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	encoder, err := zstd.NewWriter(nil,
		zstd.WithWindowSize(1<<20),
		zstd.WithLowerEncoderMem(true),
	)
	if err != nil {
		return nil, errs.Wrap(err)
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
		projects:            projects,
		projectMembers:      projectMembers,
		users:               users,
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
		userInfoCache: lrucache.NewOf[*console.UserInfo](lrucache.Options{
			Expiration: config.UserInfoValidation.CacheExpiration,
			Capacity:   config.UserInfoValidation.CacheCapacity,
		}),
		encInlineSegmentSize: encInlineSegmentSize,
		revocations:          revocations,
		config:               extendedConfig,
		versionCollector:     newVersionCollector(log),
		zstdDecoder:          decoder,
		zstdEncoder:          encoder,
		successTrackers:      successTrackers,
		placement:            placement,
	}, nil
}

// TestingNewAPIKeysEndpoint returns an endpoint suitable for testing api keys behaviour.
func TestingNewAPIKeysEndpoint(log *zap.Logger, apiKeys APIKeys) *Endpoint {
	return &Endpoint{
		log:     log,
		apiKeys: apiKeys,
	}
}

// Run manages the internal dependencies of the endpoint such as the
// success tracker.
func (endpoint *Endpoint) Run(ctx context.Context) error {
	ticker := time.NewTicker(endpoint.config.SuccessTrackerTickDuration)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			endpoint.successTrackers.BumpGeneration()
		}
	}
}

// Close closes resources.
func (endpoint *Endpoint) Close() error { return nil }

// TestSetObjectLockEnabled sets whether bucket-level Object Lock functionality should be globally enabled.
// Used for testing.
func (endpoint *Endpoint) TestSetObjectLockEnabled(enabled bool) {
	endpoint.config.ObjectLockEnabled = enabled
}

// TestSetUseBucketLevelVersioning sets whether bucket-level Object Versioning functionality should be globally enabled.
// Used for testing.
func (endpoint *Endpoint) TestSetUseBucketLevelVersioning(enabled bool) {
	endpoint.config.UseBucketLevelObjectVersioning = enabled
}

// TestSetUseBucketLevelVersioningByProjectID sets whether bucket-level Object Versioning functionality should be enabled
// for a specific project. Used for testing.
func (endpoint *Endpoint) TestSetUseBucketLevelVersioningByProjectID(projectID uuid.UUID, enabled bool) {
	if !enabled {
		delete(endpoint.config.useBucketLevelObjectVersioningProjects, projectID)
		return
	}
	endpoint.config.useBucketLevelObjectVersioningProjects[projectID] = struct{}{}
}

// ProjectInfo returns allowed ProjectInfo for the provided API key.
func (endpoint *Endpoint) ProjectInfo(ctx context.Context, req *pb.ProjectInfoRequest) (_ *pb.ProjectInfoResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	endpoint.versionCollector.collect(req.Header.UserAgent, mon.Func().ShortName())

	keyInfo, err := endpoint.validateAuth(ctx, req.Header, macaroon.Action{
		Op:   macaroon.ActionProjectInfo,
		Time: time.Now(),
	}, console.RateLimitHead)
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

	// TODO this is workaround for BeginSegment response size (it's to big)
	// We could remove more fields but they are used by RetryBeginSegmentPieces endpoint.
	// Changing RetryBeginSegmentPieces will require much more changes.
	limits := make([]*pb.AddressedOrderLimit, 0, len(satSegmentID.OriginalOrderLimits))
	for i := range satSegmentID.OriginalOrderLimits {
		limits = append(limits, &pb.AddressedOrderLimit{
			Limit: &pb.OrderLimit{
				SerialNumber:           satSegmentID.OriginalOrderLimits[i].Limit.SerialNumber,
				UplinkPublicKey:        satSegmentID.OriginalOrderLimits[i].Limit.UplinkPublicKey,
				StorageNodeId:          satSegmentID.OriginalOrderLimits[i].Limit.StorageNodeId,
				PieceId:                satSegmentID.OriginalOrderLimits[i].Limit.PieceId,
				Limit:                  satSegmentID.OriginalOrderLimits[i].Limit.Limit,
				Action:                 satSegmentID.OriginalOrderLimits[i].Limit.Action,
				PieceExpiration:        satSegmentID.OriginalOrderLimits[i].Limit.PieceExpiration,
				OrderExpiration:        satSegmentID.OriginalOrderLimits[i].Limit.OrderExpiration,
				OrderCreation:          satSegmentID.OriginalOrderLimits[i].Limit.OrderCreation,
				EncryptedMetadataKeyId: satSegmentID.OriginalOrderLimits[i].Limit.EncryptedMetadataKeyId,
				EncryptedMetadata:      satSegmentID.OriginalOrderLimits[i].Limit.EncryptedMetadata,
			},
			// not included to reduce size: StorageNodeAddress and Limit.SatelliteSignature
		})
	}
	satSegmentID.OriginalOrderLimits = limits

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

// ConvertMetabaseErr converts domain errors from metabase to appropriate rpc statuses errors.
func (endpoint *Endpoint) ConvertMetabaseErr(err error) error {
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
	case metabase.ErrObjectLock.Has(err):
		return rpcstatus.Error(rpcstatus.PermissionDenied, objectLockedErrMsg)
	case metabase.ErrObjectExpiration.Has(err):
		return rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	case metabase.ErrInvalidRequest.Has(err):
		return rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	case metabase.ErrFailedPrecondition.Has(err):
		return rpcstatus.Error(rpcstatus.FailedPrecondition, err.Error())
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
		eventkit.Bytes("macaroon-head", keyInfo.Head),
		eventkit.String("user-agent", string(header.UserAgent)),
		eventkit.String("request", name),
	}, tags...)...)
}

func (endpoint *Endpoint) getRSProto(placementID storj.PlacementConstraint) *pb.RedundancyScheme {
	rs := endpoint.config.RS.Override(endpoint.placement[placementID].EC)
	return &pb.RedundancyScheme{
		Type:             pb.RedundancyScheme_RS,
		MinReq:           int32(rs.Min),
		RepairThreshold:  int32(rs.Repair),
		SuccessThreshold: int32(rs.Success),
		Total:            int32(rs.Total),
		ErasureShareSize: rs.ErasureShareSize.Int32(),
	}
}
