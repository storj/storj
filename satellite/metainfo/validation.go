// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
	"golang.org/x/time/rate"

	"storj.io/common/encryption"
	"storj.io/common/macaroon"
	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/storj"
	"storj.io/common/time2"
	"storj.io/common/uuid"
	"storj.io/eventkit"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleauth"
	"storj.io/storj/satellite/entitlements"
	"storj.io/storj/satellite/metabase"
)

const (
	encryptedKeySize = 48

	maxRetentionDays  = 36500
	maxRetentionYears = 10

	minBucketNameLength = 3
	maxBucketNameLength = 63

	maxBucketTags          = 50
	maxBucketTagKeyChars   = 128
	maxBucketTagValueChars = 256

	unauthorizedErrMsg = "Unauthorized API credentials"
	bucketNameErrMsg   = "The specified bucket name must be at least %d and no more than %d characters long"
)

var (
	ipRegexp = regexp.MustCompile(`^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$`)

	ek = eventkit.Package()
)

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

	if endpoint.migrationModeFlag.Enabled() {
		if _, found := endpoint.config.TestingSpannerProjects[keyInfo.ProjectID]; !found {
			if !readAction(action) {
				return nil, rpcstatus.Error(rpcstatus.ResourceExhausted, "try again later")
			}
		}
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

	if endpoint.migrationModeFlag.Enabled() {
		if _, found := endpoint.config.TestingSpannerProjects[keyInfo.ProjectID]; !found {
			for _, p := range permissions {
				if !readAction(p.Action) {
					return nil, rpcstatus.Error(rpcstatus.ResourceExhausted, "try again later")
				}
			}
		}
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

func readAction(action macaroon.Action) bool {
	switch action.Op {
	case macaroon.ActionRead, macaroon.ActionList, macaroon.ActionGetObjectRetention, macaroon.ActionGetObjectLegalHold:
		return true
	}
	return false
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

	if endpoint.migrationModeFlag.Enabled() {
		if _, found := endpoint.config.TestingSpannerProjects[keyInfo.ProjectID]; !found {
			for _, p := range permissions {
				if !readAction(p.Action) {
					return nil, rpcstatus.Error(rpcstatus.ResourceExhausted, "try again later")
				}
			}
		}
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

	// we add 3 tags now, 1 in a defer, and 4 tags in checkRate, so allocate space for
	// 8 tags. the only downside to getting this number wrong is we do some unnecessary
	// allocations.
	authTags := make([]eventkit.Tag, 0, 8)
	authTags = append(authTags,
		eventkit.String("user-agent", userAgent),
		eventkit.String("project-public-id", keyInfo.ProjectPublicID.String()),
		eventkit.String("partner", string(keyInfo.UserAgent)),
	)
	defer func() {
		authTags = append(authTags,
			// this might be OK but the overall rpc might still return some other
			// code besides this one.
			eventkit.String("basic-status", rpcstatus.Code(err).String()),
		)

		ek.Event("auth", authTags...)
	}()

	if err = endpoint.checkUserStatus(ctx, keyInfo); err != nil {
		endpoint.log.Debug("user status check failed", zap.Error(err))
		return nil, nil, err
	}

	if err = endpoint.checkRate(ctx, keyInfo, rateKind, &authTags); err != nil {
		endpoint.log.Debug("rate check failed", zap.Error(err))
		return nil, nil, err
	}

	err = endpoint.handleAPIKeyTails(ctx, key, keyInfo)
	if err != nil {
		endpoint.log.Debug("api key tails check failed", zap.Error(err))
		return nil, nil, err
	}

	return key, keyInfo, nil
}

func (endpoint *Endpoint) handleAPIKeyTails(ctx context.Context, key *macaroon.APIKey, keyInfo *console.APIKeyInfo) error {
	if endpoint.keyTailsHandler == nil {
		return nil
	}

	if !keyInfo.Version.SupportsAuditability() {
		// For non-auditable keys, store tails automatically.
		combiner := endpoint.keyTailsHandler.combiner.Load()
		if combiner != nil {
			combiner.Enqueue(ctx, keyTailTask{
				rootKeyID:  keyInfo.ID,
				serialized: key.Serialize(),
				raw:        key.SerializeRaw(),
				secret:     keyInfo.Secret,
			})
		}

		return nil
	}

	// For auditable keys, check that all tails exist.
	mac, err := macaroon.ParseMacaroon(key.SerializeRaw())
	if err != nil {
		return rpcstatus.Errorf(rpcstatus.InvalidArgument, "invalid macaroon: %v", err)
	}

	tails := mac.Tails(keyInfo.Secret)
	if len(tails) <= 1 {
		return nil
	}

	tailsToCheck := tails[1:]

	results, err := endpoint.apiKeyTails.CheckExistenceBatch(ctx, tailsToCheck)
	if err != nil {
		return rpcstatus.Errorf(rpcstatus.Internal, "failed to check tail existence: %v", err)
	}

	for _, tail := range tailsToCheck {
		tailHex := hex.EncodeToString(tail)
		if !results[tailHex] {
			return rpcstatus.Errorf(rpcstatus.PermissionDenied, "unregistered tail not allowed for auditable API key")
		}
	}

	return nil
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

// validateSelfServePlacement enforces entitlements-first placement validation.
// Rules:
//  1. If entitlements are enabled, every project MUST have an entitlements row
//  2. If entitlements are enabled but project not found in entitlements: error (no fallback)
//  3. If entitlements exist but NewBucketPlacements is nil/empty: error (no fallback)
//  4. If entitlements exist with valid placements: check placement is in that list
//  5. Always verify placement exists in selfServePlacements and has no WaitlistURL
func (endpoint *Endpoint) validateSelfServePlacement(ctx context.Context, project *console.Project, placement storj.PlacementConstraint) error {
	if placementDetail, exists := endpoint.selfServePlacements[placement]; !exists || placementDetail.WaitlistURL != "" {
		return rpcstatus.Error(rpcstatus.PlacementInvalidValue, "placement not allowed")
	}

	if endpoint.entitlementsConfig.Enabled {
		feats, err := endpoint.entitlementsService.Projects().GetByPublicID(ctx, project.PublicID)
		if err != nil {
			if entitlements.ErrNotFound.Has(err) {
				// No entitlements row: fallback to global allowlist (already validated above).
				return nil
			}
			return rpcstatus.Error(rpcstatus.Internal, "unable to validate project entitlements")
		}

		if len(feats.NewBucketPlacements) == 0 || !slices.Contains(feats.NewBucketPlacements, placement) {
			return rpcstatus.Error(rpcstatus.PlacementInvalidValue, "placement not allowed")
		}
	} else {
		if project.DefaultPlacement != storj.DefaultPlacement {
			return rpcstatus.Error(rpcstatus.PlacementConflictingValues, "conflicting placement values")
		}
	}

	return nil
}

// checkRate validates whether the rate limiter has been hit for a particular project and operation.
// If the project has an operation-specific rate limit for the operation in question, that is used
// Otherwise, if the project has a basic "project-level" rate limit, that is used
// Otherwise, the global rate limit configs on the satellite are used.
// If eventTags is not nil, project rate limit tags are added to the eventTag list.
func (endpoint *Endpoint) checkRate(ctx context.Context, apiKeyInfo *console.APIKeyInfo, rateKind console.LimitKind, eventTags *[]eventkit.Tag) (err error) {
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
	case console.RateLimitPut, console.RateLimitPutNoError:
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

	if !limiter.AllowN(endpoint.rateLimiterTime(), 1) {
		if limiter.Burst() == 0 && limiter.Limit() == 0 {
			return rpcstatus.Error(rpcstatus.PermissionDenied, "All access disabled")
		}

		if eventTags != nil {
			*eventTags = append(*eventTags,
				eventkit.Bool("project-limited", true),
				eventkit.Float64("rate-limit", float64(limiter.Limit())),
				eventkit.Float64("burst-limit", float64(limiter.Burst())),
				eventkit.Int64("rate-limit-kind", int64(rateKind)),
			)
		}

		mon.Event("metainfo_rate_limit_exceeded")

		if rateKind != console.RateLimitPutNoError {
			return rpcstatus.Error(rpcstatus.ResourceExhausted, "Too Many Requests")
		}
		return nil
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

	if len(bucket) < minBucketNameLength || len(bucket) > maxBucketNameLength {
		return Error.New(bucketNameErrMsg, minBucketNameLength, maxBucketNameLength)
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

	if ipRegexp.Match(bucket) {
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

func validateBucketObjectLockStatus(bucket buckets.Bucket, retention metabase.Retention, legalHoldRequested bool) error {
	if (retention.Enabled() || legalHoldRequested) && (bucket.Versioning != buckets.VersioningEnabled || !bucket.ObjectLock.Enabled) {
		// note: AWS returns an "object lock configuration missing"
		// error for both unversioned or missing object lock
		// configuration.
		return rpcstatus.Errorf(rpcstatus.ObjectLockBucketRetentionConfigurationMissing, "cannot specify Object Lock settings when uploading into a bucket without Object Lock enabled")
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

// convertProtobufObjectLockConfig validates and converts *pb.ObjectLockConfiguration.
func convertProtobufObjectLockConfig(config *pb.ObjectLockConfiguration) (updateParams buckets.UpdateBucketObjectLockParams, err error) {
	if config == nil {
		return buckets.UpdateBucketObjectLockParams{}, rpcstatus.Error(
			rpcstatus.ObjectLockInvalidBucketRetentionConfiguration,
			"Object Lock configuration is required",
		)
	}

	if !config.Enabled {
		return buckets.UpdateBucketObjectLockParams{}, rpcstatus.Error(
			rpcstatus.ObjectLockInvalidBucketRetentionConfiguration,
			"Object Lock can't be disabled",
		)
	}

	updateParams.ObjectLockEnabled = true

	if config.DefaultRetention == nil {
		noRetention := storj.NoRetention
		updateParams.DefaultRetentionMode = doublePtr(noRetention)
		updateParams.DefaultRetentionDays = nilDoublePtr[int]()
		updateParams.DefaultRetentionYears = nilDoublePtr[int]()
	} else {
		if config.DefaultRetention.Mode == pb.Retention_INVALID {
			return buckets.UpdateBucketObjectLockParams{}, rpcstatus.Error(
				rpcstatus.ObjectLockInvalidBucketRetentionConfiguration,
				"Invalid retention mode",
			)
		}

		mode := storj.RetentionMode(config.DefaultRetention.Mode)
		updateParams.DefaultRetentionMode = doublePtr(mode)

		switch duration := config.DefaultRetention.Duration.(type) {
		case *pb.DefaultRetention_Days:
			days := int(duration.Days)
			switch {
			case days <= 0:
				return buckets.UpdateBucketObjectLockParams{}, rpcstatus.Error(
					rpcstatus.ObjectLockInvalidBucketRetentionConfiguration,
					"Days must be a positive integer",
				)
			case days > maxRetentionDays:
				return buckets.UpdateBucketObjectLockParams{}, rpcstatus.Error(
					rpcstatus.ObjectLockInvalidBucketRetentionConfiguration,
					fmt.Sprintf("Days must not exceed %d", maxRetentionDays),
				)
			}
			updateParams.DefaultRetentionDays = doublePtr(days)
			updateParams.DefaultRetentionYears = nilDoublePtr[int]()
		case *pb.DefaultRetention_Years:
			years := int(duration.Years)
			switch {
			case years <= 0:
				return buckets.UpdateBucketObjectLockParams{}, rpcstatus.Error(
					rpcstatus.ObjectLockInvalidBucketRetentionConfiguration,
					"Years must be a positive integer",
				)
			case years > maxRetentionYears:
				return buckets.UpdateBucketObjectLockParams{}, rpcstatus.Error(
					rpcstatus.ObjectLockInvalidBucketRetentionConfiguration,
					fmt.Sprintf("Years must not exceed %d", maxRetentionYears),
				)
			}
			updateParams.DefaultRetentionYears = doublePtr(years)
			updateParams.DefaultRetentionDays = nilDoublePtr[int]()
		default:
			return buckets.UpdateBucketObjectLockParams{}, rpcstatus.Error(
				rpcstatus.ObjectLockInvalidBucketRetentionConfiguration,
				"Either days or years must be specified",
			)
		}
	}

	return updateParams, nil
}

// doublePtr returns a double pointer to the given value.
func doublePtr[T any](value T) **T {
	ptr := &value
	return &ptr
}

// nilDoublePtr returns a double pointer to nil.
func nilDoublePtr[T any]() **T {
	var ptr *T
	return &ptr
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
		// don't log errors if it was user cancellation
		if !errors.Is(ctx.Err(), context.Canceled) {
			endpoint.log.Error(
				"Retrieving project bandwidth total failed; bandwidth limit won't be enforced",
				zap.Stringer("Project ID", keyInfo.ProjectID),
				zap.Error(err),
			)
		}
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

func (endpoint *Endpoint) addSegmentToUploadLimits(ctx context.Context, keyInfo *console.APIKeyInfo, segmentSize int64) {
	endpoint.addToUploadLimits(ctx, keyInfo, segmentSize, 1)
}

func (endpoint *Endpoint) addToUploadLimits(ctx context.Context, keyInfo *console.APIKeyInfo, size, segmentCount int64) {
	if err := endpoint.projectUsage.UpdateProjectStorageAndSegmentUsage(ctx, keyInfoToLimits(keyInfo), size, segmentCount); err != nil {
		// don't log errors if it was user cancellation
		if !errors.Is(ctx.Err(), context.Canceled) {
			// log it and continue. it's most likely our own fault that we couldn't
			// track it, and the only thing that will be affected is our per-project
			// bandwidth and storage limits.
			endpoint.log.Error("Could not track new project's storage and segment usage",
				zap.Stringer("Project ID", keyInfo.ProjectID),
				zap.Error(err),
			)
		}
	}
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

		// don't log errors if it was user cancellation
		if !errors.Is(ctx.Err(), context.Canceled) {
			endpoint.log.Error(
				"Updating project upload limits failed; limits won't be enforced",
				zap.Stringer("Project ID", keyInfo.ProjectID),
				zap.Error(err),
			)
		}
	}

	return nil
}

// checkEncryptedMetadata checks encrypted metadata and it's encrypted key sizes. Metadata encrypted key nonce
// is serialized to storj.Nonce automatically.
func (endpoint *Endpoint) checkEncryptedMetadataSize(userData metabase.EncryptedUserData) error {
	metadataSize := memory.Size(len(userData.EncryptedMetadata) + len(userData.EncryptedETag))
	if metadataSize > endpoint.config.MaxMetadataSize {
		return rpcstatus.Errorf(rpcstatus.InvalidArgument, "Encrypted metadata is too large, got %v, maximum allowed is %v", metadataSize, endpoint.config.MaxMetadataSize)
	}

	// verify key only if any metadata was set
	if metadataSize > 0 && len(userData.EncryptedMetadataEncryptedKey) != encryptedKeySize {
		return rpcstatus.Errorf(rpcstatus.InvalidArgument, "Encrypted metadata key size is invalid, got %v, expected %v", len(userData.EncryptedMetadataEncryptedKey), encryptedKeySize)
	}
	return nil
}

func (endpoint *Endpoint) checkObjectUploadRate(ctx context.Context, publicID uuid.UUID, bucketName []byte, objectKey []byte) error {
	if !endpoint.config.UploadLimiter.Enabled {
		return nil
	}

	if !endpoint.singleObjectUploadLimitCache.Allow(time2.Now(ctx),
		bytes.Join([][]byte{publicID[:], bucketName, objectKey}, []byte{'/'})) {
		ek.Event("single-object-upload-limit",
			eventkit.String("project-public-id", publicID.String()),
			eventkit.String("bucket", string(bucketName)),
			eventkit.Bytes("object-key", objectKey),
		)
		return rpcstatus.Error(rpcstatus.ResourceExhausted, "Too Many Requests")
	}

	return nil
}

func (endpoint *Endpoint) validateDeleteObjectsRequestSimple(req *pb.DeleteObjectsRequest) (err error) {
	bucketNameLen := len(req.Bucket)
	if bucketNameLen == 0 {
		return rpcstatus.Error(rpcstatus.BucketNameMissing, "A bucket name is required")
	}
	if err := validateBucketNameLength(req.Bucket); err != nil {
		return rpcstatus.Error(rpcstatus.BucketNameInvalid, err.Error())
	}

	numItems := len(req.Items)
	if numItems == 0 {
		return rpcstatus.Error(rpcstatus.DeleteObjectsNoItems, "The list of objects must contain at least one item")
	}
	if numItems > metabase.DeleteObjectsMaxItems {
		return rpcstatus.Error(rpcstatus.DeleteObjectsTooManyItems, "The list of objects contains too many items")
	}

	for _, item := range req.Items {
		objectKeyLen := len(item.EncryptedObjectKey)
		if objectKeyLen == 0 {
			return rpcstatus.Error(rpcstatus.ObjectKeyMissing, "An object key was not provided")
		}
		if objectKeyLen > endpoint.config.MaxEncryptedObjectKeyLength {
			return rpcstatus.Error(rpcstatus.ObjectKeyTooLong, "A provided object key is too long")
		}

		if versionLen := len(item.ObjectVersion); versionLen > 0 {
			invalid := versionLen != len(metabase.StreamVersionID{})
			if !invalid {
				invalid = metabase.StreamVersionID(item.ObjectVersion).Version() == 0
			}
			if invalid {
				return rpcstatus.Error(rpcstatus.ObjectVersionInvalid, "A provided object version is invalid")
			}
		}
	}

	return nil
}

func (endpoint *Endpoint) validateSetBucketTaggingRequestSimple(req *pb.SetBucketTaggingRequest) (err error) {
	bucketNameLen := len(req.Name)
	if bucketNameLen == 0 {
		return rpcstatus.Error(rpcstatus.BucketNameMissing, "A bucket name is required")
	}
	if err := validateBucketNameLength(req.Name); err != nil {
		return rpcstatus.Error(rpcstatus.BucketNameInvalid, err.Error())
	}

	numTags := len(req.Tags)
	if numTags > maxBucketTags {
		return rpcstatus.Error(rpcstatus.TooManyTags, "The tag set contains too many items")
	}

	keys := make(map[string]struct{}, numTags)
	for _, protoTag := range req.Tags {
		key := string(protoTag.Key)
		if _, seen := keys[key]; seen {
			return rpcstatus.Error(rpcstatus.TagKeyDuplicate, "A provided tag key is duplicated")
		}
		keys[key] = struct{}{}

		if len(key) == 0 {
			return rpcstatus.Error(rpcstatus.TagKeyInvalid, "A tag key was not provided")
		}
		if !utf8.ValidString(key) {
			return rpcstatus.Error(rpcstatus.TagKeyInvalid, "A provided tag key is not a valid UTF-8 string")
		}
		if utf8.RuneCountInString(key) > maxBucketTagKeyChars {
			return rpcstatus.Error(rpcstatus.TagKeyInvalid, "A provided tag key is too long")
		}
		for _, r := range key {
			if !isTagRuneValid(r) {
				return rpcstatus.Error(rpcstatus.TagKeyInvalid, "A provided tag key contains a disallowed character")
			}
		}

		value := string(protoTag.Value)
		if !utf8.ValidString(value) {
			return rpcstatus.Error(rpcstatus.TagValueInvalid, "A provided tag value is not a valid UTF-8 string")
		}
		if utf8.RuneCountInString(value) > maxBucketTagValueChars {
			return rpcstatus.Error(rpcstatus.TagValueInvalid, "A provided tag value is too long")
		}
		for _, r := range value {
			if !isTagRuneValid(r) {
				return rpcstatus.Error(rpcstatus.TagValueInvalid, "A provided tag value contains a disallowed character")
			}
		}
	}

	return nil
}

func isTagRuneValid(r rune) bool {
	switch r {
	case '+', '-', '.', '/', ':', '=', '@', '_':
		return true
	default:
		return unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r)
	}
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

func validateServerSideCopyFlag(flag bool, trustedUplink bool) error {
	if flag && !trustedUplink {
		return rpcstatus.Error(rpcstatus.InvalidArgument, "ServerSideCopy flag is only allowed for trusted uplink clients")
	}
	return nil
}
