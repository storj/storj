// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/spanner"
	"github.com/klauspost/compress/zstd"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
	"google.golang.org/grpc/codes"

	"storj.io/common/encryption"
	"storj.io/common/identity"
	"storj.io/common/macaroon"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/eventkit"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/attribution"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleweb"
	"storj.io/storj/satellite/entitlements"
	"storj.io/storj/satellite/eventing/eventingconfig"
	"storj.io/storj/satellite/internalpb"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metainfo/bloomrate"
	"storj.io/storj/satellite/metainfo/pointerverification"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/revocation"
	"storj.io/storj/satellite/trust"
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

	log                            *zap.Logger
	buckets                        *buckets.Service
	metabase                       *metabase.DB
	orders                         *orders.Service
	overlay                        *overlay.Service
	attributions                   attribution.DB
	pointerVerification            *pointerverification.Service
	projectUsage                   *accounting.Service
	projects                       console.Projects
	projectMembers                 console.ProjectMembers
	users                          console.Users
	apiKeys                        APIKeys
	apiKeyTails                    console.APIKeyTails
	satellite                      signing.Signer
	limiterCache                   *lrucache.ExpiringLRUOf[*rate.Limiter]
	singleObjectUploadLimitCache   *bloomrate.BloomRate
	singleObjectDownloadLimitCache *bloomrate.BloomRate
	userInfoCache                  *lrucache.ExpiringLRUOf[*console.UserInfo]
	encInlineSegmentSize           int64 // max inline segment size + encryption overhead
	revocations                    revocation.DB
	config                         Config
	migrationModeFlag              *MigrationModeFlagExtension
	versionCollector               *versionCollector
	zstdDecoder                    *zstd.Decoder
	zstdEncoder                    *zstd.Encoder
	successTrackers                *SuccessTrackers
	failureTracker                 SuccessTracker
	trustedUplinks                 *trust.TrustedPeersList
	placement                      nodeselection.PlacementDefinitions
	placementEdgeUrlOverrides      console.PlacementEdgeURLOverrides
	selfServePlacements            map[storj.PlacementConstraint]console.PlacementDetail
	nodeSelectionStats             *NodeSelectionStats
	bucketEventing                 eventingconfig.BucketLocationTopicIDMap
	entitlementsService            *entitlements.Service
	entitlementsConfig             entitlements.Config
	keyTailsHandler                *keyTailsHandler

	// rateLimiterTime is a function that returns the time to check with the rate limiter.
	// It's handy for testing purposes. It defaults to time.Now.
	rateLimiterTime func() time.Time
}

// NewEndpoint creates new metainfo endpoint instance.
func NewEndpoint(log *zap.Logger, buckets *buckets.Service, metabaseDB *metabase.DB,
	orders *orders.Service, cache *overlay.Service, attributions attribution.DB, peerIdentities overlay.PeerIdentities,
	apiKeys APIKeys, apiKeyTails console.APIKeyTails, projectUsage *accounting.Service, projects console.Projects,
	projectMembers console.ProjectMembers, users console.Users, satellite signing.Signer, revocations revocation.DB,
	successTrackers *SuccessTrackers, failureTracker SuccessTracker, trustedUplinks *trust.TrustedPeersList, config Config,
	migrationModeFlag *MigrationModeFlagExtension, placement nodeselection.PlacementDefinitions, consoleConfig consoleweb.Config,
	ordersConfig orders.Config, nodeSelectionStats *NodeSelectionStats, bucketEventing eventingconfig.BucketLocationTopicIDMap,
	entitlementsService *entitlements.Service, entitlementsConfig entitlements.Config) (
	*Endpoint, error) {
	trustedOrders := ordersConfig.TrustedOrders
	placementEdgeUrlOverrides := consoleConfig.Config.PlacementEdgeURLOverrides
	// TODO do something with too many params

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

	selfServePlacements := make(map[storj.PlacementConstraint]console.PlacementDetail)
	for _, p := range consoleConfig.Placement.SelfServeDetails {
		selfServePlacements[storj.PlacementConstraint(p.ID)] = p
	}

	e := &Endpoint{
		log:                 log,
		buckets:             buckets,
		metabase:            metabaseDB,
		orders:              orders,
		overlay:             cache,
		attributions:        attributions,
		pointerVerification: pointerverification.NewService(peerIdentities, cache, trustedUplinks, trustedOrders),
		apiKeys:             apiKeys,
		apiKeyTails:         apiKeyTails,
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
		singleObjectUploadLimitCache: bloomrate.NewBloomRate(
			config.UploadLimiter.SizeExponent,
			config.UploadLimiter.HashCount,
			rate.Every(config.UploadLimiter.SingleObjectLimit),
			config.UploadLimiter.BurstLimit),
		singleObjectDownloadLimitCache: bloomrate.NewBloomRate(
			config.DownloadLimiter.SizeExponent,
			config.DownloadLimiter.HashCount,
			rate.Every(config.DownloadLimiter.SingleObjectLimit),
			config.DownloadLimiter.BurstLimit),
		userInfoCache: lrucache.NewOf[*console.UserInfo](lrucache.Options{
			Expiration: config.UserInfoValidation.CacheExpiration,
			Capacity:   config.UserInfoValidation.CacheCapacity,
		}),
		encInlineSegmentSize:      encInlineSegmentSize,
		revocations:               revocations,
		config:                    config,
		migrationModeFlag:         migrationModeFlag,
		versionCollector:          newVersionCollector(log),
		zstdDecoder:               decoder,
		zstdEncoder:               encoder,
		successTrackers:           successTrackers,
		failureTracker:            failureTracker,
		trustedUplinks:            trustedUplinks,
		placement:                 placement,
		placementEdgeUrlOverrides: placementEdgeUrlOverrides,
		selfServePlacements:       selfServePlacements,
		rateLimiterTime:           time.Now,
		nodeSelectionStats:        nodeSelectionStats,
		bucketEventing:            bucketEventing,
		entitlementsService:       entitlementsService,
		entitlementsConfig:        entitlementsConfig,
	}
	if config.APIKeyTailsConfig.CombinerQueueEnabled {
		e.keyTailsHandler = &keyTailsHandler{
			cache: lrucache.NewOf[struct{}](lrucache.Options{
				Expiration: config.APIKeyTailsConfig.CacheExpiration,
				Capacity:   config.APIKeyTailsConfig.CacheCapacity,
				Name:       "seen_macaroon_tail_cache",
			}),
		}
	}

	return e, nil
}

// TestingNewAPIKeysEndpoint returns an endpoint suitable for testing api keys behaviour.
func TestingNewAPIKeysEndpoint(log *zap.Logger, apiKeys APIKeys) *Endpoint {
	return &Endpoint{
		log:               log,
		apiKeys:           apiKeys,
		migrationModeFlag: NewMigrationModeFlagExtension(Config{}),
	}
}

// TestingGetLimiterCache returns the limiter cache for testing purposes.
func (endpoint *Endpoint) TestingGetLimiterCache() *lrucache.ExpiringLRUOf[*rate.Limiter] {
	return endpoint.limiterCache
}

// Run manages the internal dependencies of the endpoint such as the
// success tracker.
func (endpoint *Endpoint) Run(ctx context.Context) error {
	successTicker := time.NewTicker(endpoint.config.SuccessTrackerTickDuration)
	defer successTicker.Stop()
	failureTicker := time.NewTicker(endpoint.config.FailureTrackerTickDuration)
	defer failureTicker.Stop()

	if endpoint.config.APIKeyTailsConfig.CombinerQueueEnabled && endpoint.keyTailsHandler != nil {
		endpoint.initTailsCombiner(ctx)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-successTicker.C:
			endpoint.successTrackers.BumpGeneration()
		case <-failureTicker.C:
			endpoint.failureTracker.BumpGeneration()
		}
	}
}

// Close closes resources.
func (endpoint *Endpoint) Close() error {
	if endpoint.keyTailsHandler != nil {
		combiner := endpoint.keyTailsHandler.combiner.Load()
		if combiner != nil {
			combiner.Close()
		}
	}

	return nil
}

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

// TestSelfServePlacementEnabled sets whether self-serve placement should be enabled.
func (endpoint *Endpoint) TestSelfServePlacementEnabled(enabled bool) {
	endpoint.config.SelfServePlacementSelectEnabled = enabled
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

	project, err := endpoint.projects.Get(ctx, keyInfo.ProjectID)
	if err != nil {
		return nil, err
	}

	info := &pb.ProjectInfoResponse{
		ProjectPublicId:  keyInfo.ProjectPublicID.Bytes(),
		ProjectCreatedAt: project.CreatedAt,
		ProjectSalt:      salt,
	}

	if endpoint.config.SendEdgeUrlOverrides {
		if edgeURLs, ok := endpoint.placementEdgeUrlOverrides.Get(project.DefaultPlacement); ok {
			info.EdgeUrlOverrides = &pb.EdgeUrlOverrides{
				AuthService:        []byte(edgeURLs.AuthService),
				PublicLinksharing:  []byte(edgeURLs.PublicLinksharing),
				PrivateLinksharing: []byte(edgeURLs.InternalLinksharing),
			}
		}
	}

	return info, nil
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
		return nil, endpoint.ConvertKnownErrWithMessage(err, "Failed to revoke API key")
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
		return nil, rpcstatus.Wrap(rpcstatus.Internal, err)
	}

	encodedStreamID, err := pb.Marshal(signedStreamID)
	if err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Internal, err)
	}

	streamID, err = storj.StreamIDFromBytes(encodedStreamID)
	if err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Internal, err)
	}
	return streamID, nil
}

func (endpoint *Endpoint) packSegmentID(ctx context.Context, satSegmentID *internalpb.SegmentID) (segmentID storj.SegmentID, err error) {
	defer mon.Task()(&ctx)(&err)

	if satSegmentID == nil {
		return nil, rpcstatus.Error(rpcstatus.Internal, "unable to create segment id")
	}

	// remove satellite signature from limits to reduce response size
	// signature is not needed here, because segment id is signed by satellite
	originalOrderLimits := make([]*pb.AddressedOrderLimit, len(satSegmentID.OriginalOrderLimits))
	for i, alimit := range satSegmentID.OriginalOrderLimits {
		originalOrderLimits[i] = &pb.AddressedOrderLimit{
			StorageNodeAddress: alimit.StorageNodeAddress,
			Limit: &pb.OrderLimit{
				SerialNumber:           alimit.Limit.SerialNumber,
				SatelliteId:            alimit.Limit.SatelliteId,
				UplinkPublicKey:        alimit.Limit.UplinkPublicKey,
				StorageNodeId:          alimit.Limit.StorageNodeId,
				PieceId:                alimit.Limit.PieceId,
				Limit:                  alimit.Limit.Limit,
				PieceExpiration:        alimit.Limit.PieceExpiration,
				Action:                 alimit.Limit.Action,
				OrderExpiration:        alimit.Limit.OrderExpiration,
				OrderCreation:          alimit.Limit.OrderCreation,
				EncryptedMetadataKeyId: alimit.Limit.EncryptedMetadataKeyId,
				EncryptedMetadata:      alimit.Limit.EncryptedMetadata,
				// don't copy satellite signature
			},
		}
	}

	satSegmentID.OriginalOrderLimits = originalOrderLimits

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

// ConvertMetabaseErr converts known domain errors to appropriate rpc statuses errors.
func (endpoint *Endpoint) ConvertMetabaseErr(err error) error {
	return endpoint.ConvertKnownErrWithMessage(err, "internal error")
}

// ConvertKnownErr converts known domain errors to appropriate rpc statuses errors.
func (endpoint *Endpoint) ConvertKnownErr(err error) error {
	return endpoint.ConvertKnownErrWithMessage(err, "internal error")
}

// ConvertKnownErrWithMessage converts known domain errors to appropriate rpc statuses errors with
// a custom message.
func (endpoint *Endpoint) ConvertKnownErrWithMessage(err error, message string) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, context.Canceled):
		return rpcstatus.Wrap(rpcstatus.Canceled, context.Canceled)
	case errors.Is(err, context.DeadlineExceeded):
		return rpcstatus.Wrap(rpcstatus.DeadlineExceeded, context.DeadlineExceeded)
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
		return rpcstatus.Error(rpcstatus.ObjectLockObjectProtected, objectLockedErrMsg)
	case metabase.ErrObjectExpiration.Has(err):
		return rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	case metabase.ErrInvalidRequest.Has(err):
		return rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	case metabase.ErrFailedPrecondition.Has(err):
		return rpcstatus.Error(rpcstatus.FailedPrecondition, err.Error())
	case metabase.ErrUnimplemented.Has(err):
		return rpcstatus.Error(rpcstatus.Unimplemented, err.Error())
	case metabase.ErrObjectAlreadyExists.Has(err):
		return rpcstatus.Error(rpcstatus.AlreadyExists, err.Error())
	case metabase.ErrPendingObjectMissing.Has(err):
		return rpcstatus.Error(rpcstatus.NotFound, err.Error())
	case metabase.ErrPermissionDenied.Has(err):
		return rpcstatus.Error(rpcstatus.PermissionDenied, err.Error())
	case spanner.ErrCode(err) == codes.Canceled:
		// TODO(spanner): it's far from perfect we should be handling this on lower level
		return rpcstatus.Wrap(rpcstatus.Canceled, context.Canceled)
	default:
		endpoint.log.Error(message, zap.Error(err))
		return rpcstatus.Error(rpcstatus.Internal, message)
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

// TestingSetRSConfig set endpoint RS config for testing.
func (endpoint *Endpoint) TestingSetRSConfig(rs RSConfig) {
	endpoint.config.RS = rs
}

// TestingSetRateLimiterTime sets the time function used by the rate limiter.
func (endpoint *Endpoint) TestingSetRateLimiterTime(time func() time.Time) {
	endpoint.rateLimiterTime = time
}

// TestingAddTrustedUplink is a helper function for tests to add a trusted uplink.
func (endpoint *Endpoint) TestingAddTrustedUplink(id storj.NodeID) {
	endpoint.trustedUplinks.TestingAddTrustedUplink(id)
}

func (endpoint *Endpoint) uplinkPeer(ctx context.Context) (peer *identity.PeerIdentity, trusted bool, err error) {
	peer, err = identity.PeerIdentityFromContext(ctx)
	if err != nil {
		// N.B. jeff thinks this is a bad idea but jt convinced him
		return nil, false, rpcstatus.Errorf(rpcstatus.Unauthenticated, "unable to get peer identity: %w", err)
	}

	return peer, endpoint.trustedUplinks.IsTrusted(peer.ID), nil
}
