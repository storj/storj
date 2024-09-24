// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"bytes"
	"context"
	"crypto/subtle"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"storj.io/common/encryption"
	"storj.io/common/errs2"
	"storj.io/common/macaroon"
	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/eventkit"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleauth"
	"storj.io/storj/satellite/metabase"
)

const encryptedKeySize = 48

var (
	ipRegexp           = regexp.MustCompile(`^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$`)
	unauthorizedErrMsg = "Unauthorized API credentials"
)

var ek = eventkit.Package()

func getAPIKey(ctx context.Context, header *pb.RequestHeader) (key *macaroon.APIKey, err error) {
	defer mon.Task()(&ctx)(&err)
	if header != nil {
		return macaroon.ParseRawAPIKey(header.ApiKey)
	}

	keyData, ok := consoleauth.GetAPIKey(ctx)
	if !ok {
		return nil, errs.New("missing credentials")
	}

	return macaroon.ParseAPIKey(string(keyData))
}

// validateAuth validates things like API key, user permissions and rate limit and always returns valid rpc error.
func (endpoint *Endpoint) validateAuth(ctx context.Context, header *pb.RequestHeader, action macaroon.Action, rateLimitKind console.LimitKind) (_ *console.APIKeyInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	key, keyInfo, err := endpoint.validateBasic(ctx, header, rateLimitKind)
	if err != nil {
		return nil, err
	}

	err = key.Check(ctx, keyInfo.Secret, keyInfo.Version, action, endpoint.revocations)
	if err != nil {
		endpoint.log.Debug("unauthorized request", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.PermissionDenied, unauthorizedErrMsg)
	}

	return keyInfo, nil
}

// VerifyPermission specifies one permission that is required by an endpoint.
type VerifyPermission struct {
	Action          macaroon.Action
	ActionPermitted *bool
	Optional        bool
}

// ValidateAuthN validates things like API keys, rate limit and user permissions
// for each permission from permissions. It returns an error for the first
// required (not optional) permission that the check fails for. There must be at
// least one required (not optional) permission. In case all permissions are
// optional, it will return an error. It always returns valid RPC errors.
func (endpoint *Endpoint) ValidateAuthN(ctx context.Context, header *pb.RequestHeader, rateLimitKind console.LimitKind, permissions ...VerifyPermission) (_ *console.APIKeyInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	allOptional := true

	for _, p := range permissions {
		if !p.Optional {
			allOptional = false
			break
		}
	}

	if allOptional {
		return nil, rpcstatus.Error(rpcstatus.Internal, "All permissions are optional")
	}

	key, keyInfo, err := endpoint.validateBasic(ctx, header, rateLimitKind)
	if err != nil {
		return nil, err
	}

	for _, p := range permissions {
		err = key.Check(ctx, keyInfo.Secret, keyInfo.Version, p.Action, endpoint.revocations)
		if p.ActionPermitted != nil {
			*p.ActionPermitted = err == nil
		}
		if err != nil && !p.Optional {
			endpoint.log.Debug("unauthorized request", zap.Error(err))
			return nil, rpcstatus.Error(rpcstatus.PermissionDenied, unauthorizedErrMsg)
		}
	}

	return keyInfo, nil
}

// ValidateAuthAny validates things like API keys, rate limit and user permissions.
// At least one required (not optional) permission must be permitted.
// If not, an error is returned, and optional actions aren't checked.
// It always returns valid RPC errors.
func (endpoint *Endpoint) ValidateAuthAny(ctx context.Context, header *pb.RequestHeader, rateLimitKind console.LimitKind, permissions ...VerifyPermission) (_ *console.APIKeyInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(permissions) == 0 {
		return nil, rpcstatus.Error(rpcstatus.Internal, "No permissions to check")
	}

	var optional, required []VerifyPermission
	for _, p := range permissions {
		if p.Optional {
			optional = append(optional, p)
			continue
		}
		required = append(required, p)
	}
	if len(required) == 0 {
		return nil, rpcstatus.Error(rpcstatus.Internal, "All permissions are optional")
	}

	key, keyInfo, err := endpoint.validateBasic(ctx, header, rateLimitKind)
	if err != nil {
		return nil, err
	}

	var combinedErrs error
	for _, p := range required {
		err = key.Check(ctx, keyInfo.Secret, keyInfo.Version, p.Action, endpoint.revocations)
		if err == nil {
			combinedErrs = nil
			break
		}
		combinedErrs = errs.Combine(combinedErrs, err)
	}
	if combinedErrs != nil {
		endpoint.log.Debug("unauthorized request", zap.Error(combinedErrs))
		return nil, rpcstatus.Error(rpcstatus.PermissionDenied, unauthorizedErrMsg)
	}

	for _, p := range optional {
		*p.ActionPermitted = key.Check(ctx, keyInfo.Secret, keyInfo.Version, p.Action, endpoint.revocations) == nil
	}

	return keyInfo, nil
}

func (endpoint *Endpoint) validateBasic(ctx context.Context, header *pb.RequestHeader, rateKind console.LimitKind) (_ *macaroon.APIKey, _ *console.APIKeyInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	key, err := getAPIKey(ctx, header)
	if err != nil {
		endpoint.log.Debug("invalid request", zap.Error(err))
		return nil, nil, rpcstatus.Error(rpcstatus.InvalidArgument, "Invalid API credentials")
	}

	keyInfo, err := endpoint.apiKeys.GetByHead(ctx, key.Head())
	if err != nil {
		endpoint.log.Debug("unauthorized request", zap.Error(err))
		return nil, nil, rpcstatus.Error(rpcstatus.PermissionDenied, unauthorizedErrMsg)
	}

	userAgent := ""
	if keyInfo.UserAgent != nil {
		userAgent = string(keyInfo.UserAgent)
	}
	ek.Event("auth",
		eventkit.String("user-agent", userAgent),
		eventkit.String("project", keyInfo.ProjectID.String()),
		eventkit.String("partner", string(keyInfo.UserAgent)),
	)

	if err = endpoint.checkUserStatus(ctx, keyInfo); err != nil {
		endpoint.log.Debug("user status check failed", zap.Error(err))
		return nil, nil, err
	}

	if err = endpoint.checkRate(ctx, keyInfo, rateKind); err != nil {
		endpoint.log.Debug("rate check failed", zap.Error(err))
		return nil, nil, err
	}

	return key, keyInfo, nil
}

func (endpoint *Endpoint) validateRevoke(ctx context.Context, header *pb.RequestHeader, macToRevoke *macaroon.Macaroon) (_ *console.APIKeyInfo, err error) {
	defer mon.Task()(&ctx)(&err)
	key, keyInfo, err := endpoint.validateBasic(ctx, header, console.RateLimitPut)
	if err != nil {
		return nil, err
	}

	// The macaroon to revoke must be valid with the same secret as the key.
	if !macToRevoke.Validate(keyInfo.Secret) {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, "Macaroon to revoke invalid")
	}

	keyTail := key.Tail()
	tails := macToRevoke.Tails(keyInfo.Secret)

	// A macaroon cannot revoke itself. So we only check len(tails-1), skipping
	// the final tail.  To be valid, the final tail of the auth key must be
	// contained within the checked tails of the macaroon we want to revoke.
	for i := 0; i < len(tails)-1; i++ {
		if subtle.ConstantTimeCompare(tails[i], keyTail) == 1 {
			return keyInfo, nil
		}
	}
	return nil, rpcstatus.Error(rpcstatus.PermissionDenied, "Unauthorized attempt to revoke macaroon")
}

// checkRate validates whether the rate limiter has been hit for a particular project and operation.
// If the project has an operation-specific rate limit for the operation in question, that is used
// Otherwise, if the project has a basic "project-level" rate limit, that is used
// Otherwise, the global rate limit configs on the satellite are used.
func (endpoint *Endpoint) checkRate(ctx context.Context, apiKeyInfo *console.APIKeyInfo, rateKind console.LimitKind) (err error) {
	defer mon.Task()(&ctx)(&err)
	if !endpoint.config.RateLimiter.Enabled {
		return nil
	}

	var (
		rateLimit  rate.Limit
		burstLimit int
		limiterKey = apiKeyInfo.ProjectID.String()
	)
	// checkSetRate is a helper function for validating nullable rate/burst values, and overriding `rateLimit` and `burstLimit` if needed
	checkSetRate := func(newRate, newBurst *int, keySuffix string) {
		overridden := false
		if newRate != nil {
			rateLimit = rate.Limit(*newRate)
			burstLimit = *newRate
			overridden = true
		}
		if newBurst != nil {
			burstLimit = *newBurst
			overridden = true
		}
		if overridden {
			// only use suffix in rate limiter key if override occurs
			limiterKey = apiKeyInfo.ProjectID.String() + keySuffix
		}
	}

	// set default value to global config, overridden by project.rate_limit and project.burst_limit if provided
	rateLimit = rate.Limit(endpoint.config.RateLimiter.Rate)
	burstLimit = int(endpoint.config.RateLimiter.Rate)
	checkSetRate(apiKeyInfo.ProjectRateLimit, apiKeyInfo.ProjectBurstLimit, "")

	// update rate limit values if user has custom rate limits for the provided operation
	switch rateKind {
	case console.RateLimitHead:
		checkSetRate(apiKeyInfo.ProjectRateLimitHead, apiKeyInfo.ProjectBurstLimitHead, "-head")
	case console.RateLimitGet:
		checkSetRate(apiKeyInfo.ProjectRateLimitGet, apiKeyInfo.ProjectBurstLimitGet, "-get")
	case console.RateLimitPut:
		checkSetRate(apiKeyInfo.ProjectRateLimitPut, apiKeyInfo.ProjectBurstLimitPut, "-put")
	case console.RateLimitList:
		checkSetRate(apiKeyInfo.ProjectRateLimitList, apiKeyInfo.ProjectBurstLimitList, "-list")
	case console.RateLimitDelete:
		checkSetRate(apiKeyInfo.ProjectRateLimitDelete, apiKeyInfo.ProjectBurstLimitDelete, "-delete")
	default: // invalid rate limit kind passed in, but safe to proceed with global or project defaults
	}

	limiter, err := endpoint.limiterCache.Get(ctx, limiterKey, func() (*rate.Limiter, error) {
		return rate.NewLimiter(rateLimit, burstLimit), nil
	})
	if err != nil {
		return rpcstatus.Error(rpcstatus.Unavailable, err.Error())
	}

	if !limiter.Allow() {
		if limiter.Burst() == 0 && limiter.Limit() == 0 {
			return rpcstatus.Error(rpcstatus.PermissionDenied, "All access disabled")
		}
		endpoint.log.Warn("too many requests for project",
			zap.Stringer("Project Public ID", apiKeyInfo.ProjectPublicID),
			zap.Float64("rate limit", float64(limiter.Limit())),
			zap.Float64("burst limit", float64(limiter.Burst())),
			zap.Int("rate limit kind", int(rateKind)))

		mon.Event("metainfo_rate_limit_exceeded") //mon:locked

		return rpcstatus.Error(rpcstatus.ResourceExhausted, "Too Many Requests")
	}

	return nil
}

// checkUserStatus validates whether the user associated with keyInfo is active.
func (endpoint *Endpoint) checkUserStatus(ctx context.Context, keyInfo *console.APIKeyInfo) (err error) {
	defer mon.Task()(&ctx)(&err)

	if !endpoint.config.UserInfoValidation.Enabled {
		return nil
	}

	info, err := endpoint.userInfoCache.Get(ctx, keyInfo.ProjectID.String(), func() (*console.UserInfo, error) {
		return endpoint.users.GetUserInfoByProjectID(ctx, keyInfo.ProjectID)
	})
	if err != nil {
		endpoint.log.Error("internal", zap.Error(err))
		return rpcstatus.Error(rpcstatus.Internal, "unable to get user info")
	}

	if info.Status != console.Active {
		return rpcstatus.Error(rpcstatus.PermissionDenied, "User is not active")
	}
	return nil
}

func validateBucketNameLength(bucket []byte) (err error) {
	if len(bucket) == 0 {
		return Error.Wrap(buckets.ErrNoBucket.New(""))
	}

	if len(bucket) < 3 || len(bucket) > 63 {
		return Error.New("bucket name must be at least 3 and no more than 63 characters long")
	}

	return nil
}

func validateBucketName(bucket []byte) error {
	if err := validateBucketNameLength(bucket); err != nil {
		return err
	}

	// Regexp not used because benchmark shows it will be slower for valid bucket names
	// https://gist.github.com/mniewrzal/49de3af95f36e63e88fac24f565e444c
	labels := bytes.Split(bucket, []byte("."))
	for _, label := range labels {
		err := validateBucketLabel(label)
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

	if !isLowerLetter(label[len(label)-1]) && !isDigit(label[len(label)-1]) {
		return Error.New("bucket label must end with a lowercase letter or number")
	}

	for i := 1; i < len(label)-1; i++ {
		if !isLowerLetter(label[i]) && !isDigit(label[i]) && (label[i] != '-') && (label[i] != '.') {
			return Error.New("bucket name must contain only lowercase letters, numbers or hyphens")
		}
	}

	return nil
}

func validateObjectVersion(version []byte) error {
	if len(version) != 0 && len(version) != 16 {
		return Error.New("invalid object version length")
	}
	return nil
}

func validateRetention(pbRetention *pb.Retention) error {
	if pbRetention == nil {
		return nil
	}
	switch pbRetention.Mode {
	case pb.Retention_COMPLIANCE, pb.Retention_GOVERNANCE:
		switch {
		case pbRetention.RetainUntil.IsZero():
			return errs.New("retention period expiration time must be set if retention is set")
		case pbRetention.RetainUntil.Before(time.Now()):
			return errs.New("retention period expiration time must not be in the past")
		}
	default:
		return errs.New("invalid retention mode %d", pbRetention.Mode)
	}
	return nil
}

type bucketRequest interface{ GetBucket() []byte }
type newBucketRequest interface{ GetNewBucket() []byte }
type objectVersionRequest interface{ GetObjectVersion() []byte }
type retentionRequest interface{ GetRetention() *pb.Retention }

// validateRequestSimple performs trivial validation of request fields.
//
// It returns an RPC error if any of the following are true:
//
// - The value returned by Bucket() or NewBucket() has an incorrect length.
//
// - The value returned by ObjectVersion() has an incorrect length.
//
// - The value returned by Retention() does not represent a valid retention configuration.
func validateRequestSimple(req any) (err error) {
	if req, ok := req.(bucketRequest); ok {
		if err := validateBucketNameLength(req.GetBucket()); err != nil {
			return rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
		}
	}
	if req, ok := req.(newBucketRequest); ok {
		if err := validateBucketNameLength(req.GetNewBucket()); err != nil {
			return rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
		}
	}
	if req, ok := req.(objectVersionRequest); ok {
		if err := validateObjectVersion(req.GetObjectVersion()); err != nil {
			return rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
		}
	}
	if req, ok := req.(retentionRequest); ok {
		if err := validateRetention(req.GetRetention()); err != nil {
			return rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
		}
	}
	return nil
}

// protobufRetentionToMetabase converts *pb.Retention to metabase.Retention.
func protobufRetentionToMetabase(pbRetention *pb.Retention) metabase.Retention {
	if pbRetention == nil {
		return metabase.Retention{}
	}
	return metabase.Retention{
		Mode:        storj.RetentionMode(pbRetention.Mode),
		RetainUntil: pbRetention.RetainUntil,
	}
}

func isLowerLetter(r byte) bool {
	return r >= 'a' && r <= 'z'
}

func isDigit(r byte) bool {
	return r >= '0' && r <= '9'
}

func (endpoint *Endpoint) validateRemoteSegment(ctx context.Context, commitRequest metabase.CommitSegment, originalLimits []*pb.OrderLimit) (err error) {
	defer mon.Task()(&ctx)(&err)

	if len(originalLimits) == 0 {
		return Error.New("no order limits")
	}
	if len(originalLimits) != int(commitRequest.Redundancy.TotalShares) {
		return Error.New("invalid no order limit for piece")
	}

	maxAllowed, err := encryption.CalcEncryptedSize(endpoint.config.MaxSegmentSize.Int64(), storj.EncryptionParameters{
		CipherSuite: storj.EncAESGCM,
		BlockSize:   128, // intentionally low block size to allow maximum possible encryption overhead
	})
	if err != nil {
		return err
	}

	if int64(commitRequest.EncryptedSize) > maxAllowed || commitRequest.EncryptedSize < 0 {
		return Error.New("encrypted segment size %v is out of range, maximum allowed is %v", commitRequest.EncryptedSize, maxAllowed)
	}

	// TODO more validation for plain size and plain offset
	if commitRequest.PlainSize > commitRequest.EncryptedSize {
		return Error.New("plain segment size %v is out of range, maximum allowed is %v", commitRequest.PlainSize, commitRequest.EncryptedSize)
	}

	pieceNums := make(map[uint16]struct{})
	nodeIds := make(map[storj.NodeID]struct{})
	deriver := commitRequest.RootPieceID.Deriver()
	for _, piece := range commitRequest.Pieces {
		if int(piece.Number) >= len(originalLimits) {
			return Error.New("invalid piece number")
		}

		limit := originalLimits[piece.Number]
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

		derivedPieceID := deriver.Derive(piece.StorageNode, int32(piece.Number))
		if limit.PieceId.IsZero() || limit.PieceId != derivedPieceID {
			return Error.New("invalid order limit piece id")
		}
		if piece.StorageNode != limit.StorageNodeId {
			return Error.New("piece NodeID != order limit NodeID")
		}

		if _, ok := pieceNums[piece.Number]; ok {
			return Error.New("piece num %d is duplicated", piece.Number)
		}

		if _, ok := nodeIds[piece.StorageNode]; ok {
			return Error.New("node id %s for piece num %d is duplicated", piece.StorageNode.String(), piece.Number)
		}

		pieceNums[piece.Number] = struct{}{}
		nodeIds[piece.StorageNode] = struct{}{}
	}

	return nil
}

func (endpoint *Endpoint) checkDownloadLimits(ctx context.Context, keyInfo *console.APIKeyInfo) error {
	if exceeded, limit, err := endpoint.projectUsage.ExceedsBandwidthUsage(ctx, keyInfoToLimits(keyInfo)); err != nil {
		if errs2.IsCanceled(err) {
			return rpcstatus.Wrap(rpcstatus.Canceled, err)
		}

		endpoint.log.Error(
			"Retrieving project bandwidth total failed; bandwidth limit won't be enforced",
			zap.Stringer("Project ID", keyInfo.ProjectID),
			zap.Error(err),
		)
	} else if exceeded {
		if limit > 0 {
			endpoint.log.Warn("Monthly bandwidth limit exceeded",
				zap.Stringer("Limit", limit),
				zap.Stringer("Project ID", keyInfo.ProjectID),
			)
		}
		return rpcstatus.Error(rpcstatus.ResourceExhausted, "Exceeded Usage Limit")
	}
	return nil
}

func (endpoint *Endpoint) checkUploadLimits(ctx context.Context, keyInfo *console.APIKeyInfo) error {
	return endpoint.checkUploadLimitsForNewObject(ctx, keyInfo, 1, 1)
}

func (endpoint *Endpoint) checkUploadLimitsForNewObject(
	ctx context.Context, keyInfo *console.APIKeyInfo, newObjectSize int64, newObjectSegmentCount int64,
) error {
	limit := endpoint.projectUsage.ExceedsUploadLimits(ctx, newObjectSize, newObjectSegmentCount, keyInfoToLimits(keyInfo))
	if limit.ExceedsSegments {
		if limit.SegmentsLimit > 0 {
			endpoint.log.Warn("Segment limit exceeded",
				zap.String("Limit", strconv.Itoa(int(limit.SegmentsLimit))),
				zap.Stringer("Project ID", keyInfo.ProjectID),
			)
		}
		return rpcstatus.Error(rpcstatus.ResourceExhausted, "Exceeded Segments Limit")
	}

	if limit.ExceedsStorage {
		if limit.StorageLimit > 0 {
			endpoint.log.Warn("Storage limit exceeded",
				zap.String("Limit", strconv.Itoa(limit.StorageLimit.Int())),
				zap.Stringer("Project ID", keyInfo.ProjectID),
			)
		}
		return rpcstatus.Error(rpcstatus.ResourceExhausted, "Exceeded Storage Limit")
	}

	return nil
}

func (endpoint *Endpoint) addSegmentToUploadLimits(ctx context.Context, keyInfo *console.APIKeyInfo, segmentSize int64) error {
	return endpoint.addToUploadLimits(ctx, keyInfo, segmentSize, 1)
}

func (endpoint *Endpoint) addToUploadLimits(ctx context.Context, keyInfo *console.APIKeyInfo, size, segmentCount int64) error {
	if err := endpoint.projectUsage.UpdateProjectStorageAndSegmentUsage(ctx, keyInfoToLimits(keyInfo), size, segmentCount); err != nil {
		if errs2.IsCanceled(err) {
			return rpcstatus.Wrap(rpcstatus.Canceled, err)
		}

		// log it and continue. it's most likely our own fault that we couldn't
		// track it, and the only thing that will be affected is our per-project
		// bandwidth and storage limits.
		endpoint.log.Error("Could not track new project's storage and segment usage",
			zap.Stringer("Project ID", keyInfo.ProjectID),
			zap.Error(err),
		)
	}

	return nil
}

func (endpoint *Endpoint) addStorageUsageUpToLimit(ctx context.Context, keyInfo *console.APIKeyInfo, storage int64, segments int64) (err error) {
	err = endpoint.projectUsage.AddProjectUsageUpToLimit(ctx, keyInfo.ProjectID, storage, segments, keyInfoToLimits(keyInfo))

	if err != nil {
		if accounting.ErrProjectLimitExceeded.Has(err) {
			endpoint.log.Warn("Upload limit exceeded",
				zap.Stringer("Project ID", keyInfo.ProjectID),
				zap.Error(err),
			)
			return rpcstatus.Error(rpcstatus.ResourceExhausted, err.Error())
		}

		if errs2.IsCanceled(err) {
			return rpcstatus.Wrap(rpcstatus.Canceled, err)
		}

		endpoint.log.Error(
			"Updating project upload limits failed; limits won't be enforced",
			zap.Stringer("Project ID", keyInfo.ProjectID),
			zap.Error(err),
		)
	}

	return nil
}

// checkEncryptedMetadata checks encrypted metadata and it's encrypted key sizes. Metadata encrypted key nonce
// is serialized to storj.Nonce automatically.
func (endpoint *Endpoint) checkEncryptedMetadataSize(encryptedMetadata, encryptedKey []byte) error {
	metadataSize := memory.Size(len(encryptedMetadata))
	if metadataSize > endpoint.config.MaxMetadataSize {
		return rpcstatus.Errorf(rpcstatus.InvalidArgument, "Encrypted metadata is too large, got %v, maximum allowed is %v", metadataSize, endpoint.config.MaxMetadataSize)
	}

	// verify key only if any metadata was set
	if metadataSize > 0 && len(encryptedKey) != encryptedKeySize {
		return rpcstatus.Errorf(rpcstatus.InvalidArgument, "Encrypted metadata key size is invalid, got %v, expected %v", len(encryptedKey), encryptedKeySize)
	}
	return nil
}

func (endpoint *Endpoint) checkObjectUploadRate(ctx context.Context, projectID uuid.UUID, bucketName []byte, objectKey []byte) error {
	if !endpoint.config.UploadLimiter.Enabled {
		return nil
	}

	limited := true
	// if object location is in cache it means that we won't allow to upload yet here,
	// if it's not or internally key expired we are good to go
	key := strings.Join([]string{string(projectID[:]), string(bucketName), string(objectKey)}, "/")
	_, _ = endpoint.singleObjectLimitCache.Get(ctx, key, func() (struct{}, error) {
		limited = false
		return struct{}{}, nil
	})
	if limited {
		return rpcstatus.Error(rpcstatus.ResourceExhausted, "Too Many Requests")
	}

	return nil
}

func keyInfoToLimits(keyInfo *console.APIKeyInfo) accounting.ProjectLimits {
	if keyInfo == nil {
		return accounting.ProjectLimits{}
	}

	return accounting.ProjectLimits{
		ProjectID: keyInfo.ProjectID,
		Bandwidth: keyInfo.ProjectBandwidthLimit,
		Usage:     keyInfo.ProjectStorageLimit,
		Segments:  keyInfo.ProjectSegmentsLimit,

		RateLimit:  keyInfo.ProjectRateLimit,
		BurstLimit: keyInfo.ProjectBurstLimit,
	}
}
