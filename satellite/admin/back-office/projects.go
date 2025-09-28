// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package admin

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/private/api"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/console"
)

// NullableLimitValue is the value used to indicate that a limit should be set to null.
const NullableLimitValue = -1

// Project contains the information and configurations of a project.
type Project struct {
	ID               uuid.UUID                 `json:"-"`
	PublicID         uuid.UUID                 `json:"id"`
	Name             string                    `json:"name"`
	Description      string                    `json:"description"`
	UserAgent        string                    `json:"userAgent"`
	Owner            User                      `json:"owner"`
	CreatedAt        time.Time                 `json:"createdAt"`
	DefaultPlacement storj.PlacementConstraint `json:"defaultPlacement"`

	// RateLimit is `nil` when satellite applies the configured default rate limit.
	RateLimit *int `json:"rateLimit"`
	// BurstLimit is `nil` when satellite applies the configured default burst limit.
	BurstLimit       *int `json:"burstLimit"`
	RateLimitHead    *int `json:"rateLimitHead"`
	BurstLimitHead   *int `json:"burstLimitHead"`
	RateLimitGet     *int `json:"rateLimitGet"`
	BurstLimitGet    *int `json:"burstLimitGet"`
	RateLimitPut     *int `json:"rateLimitPut"`
	BurstLimitPut    *int `json:"burstLimitPut"`
	RateLimitDelete  *int `json:"rateLimitDelete"`
	BurstLimitDelete *int `json:"burstLimitDelete"`
	RateLimitList    *int `json:"rateLimitList"`
	BurstLimitList   *int `json:"burstLimitList"`
	// Maxbuckets is `nil` when satellite applies the configured default max buckets.
	MaxBuckets *int `json:"maxBuckets"`
	ProjectUsageLimits[*int64]
}

// ProjectUsageLimits holds project usage limits and current usage. It uses generics for allowing
// to report the limits fields with nil values when they are read from the DB projects table.
//
// StorageUsed and SegmentUsed are nil if there was an error connecting to the Redis
// live accounting cache.
type ProjectUsageLimits[T ~int64 | *int64] struct {
	BandwidthLimit        T      `json:"bandwidthLimit"`
	UserSetBandwidthLimit *int64 `json:"userSetBandwidthLimit"`
	BandwidthUsed         int64  `json:"bandwidthUsed"`
	StorageLimit          T      `json:"storageLimit"`
	UserSetStorageLimit   *int64 `json:"userSetStorageLimit"`
	StorageUsed           *int64 `json:"storageUsed"`
	SegmentLimit          T      `json:"segmentLimit"`
	SegmentUsed           *int64 `json:"segmentUsed"`
}

// ProjectLimitsUpdateRequest contains all limit values to be updated.
type ProjectLimitsUpdateRequest struct {
	MaxBuckets     *int   `json:"maxBuckets"`
	StorageLimit   *int64 `json:"storageLimit"`
	BandwidthLimit *int64 `json:"bandwidthLimit"`
	SegmentLimit   *int64 `json:"segmentLimit"`
	RateLimit      *int   `json:"rateLimit"`
	BurstLimit     *int   `json:"burstLimit"`
	// the following limits are nullable; setting them to 0
	// sets them to null in the DB
	UserSetStorageLimit   *int64 `json:"userSetStorageLimit"`
	UserSetBandwidthLimit *int64 `json:"userSetBandwidthLimit"`
	RateLimitHead         *int   `json:"rateLimitHead"`
	BurstLimitHead        *int   `json:"burstLimitHead"`
	RateLimitGet          *int   `json:"rateLimitGet"`
	BurstLimitGet         *int   `json:"burstLimitGet"`
	RateLimitPut          *int   `json:"rateLimitPut"`
	BurstLimitPut         *int   `json:"burstLimitPut"`
	RateLimitDelete       *int   `json:"rateLimitDelete"`
	BurstLimitDelete      *int   `json:"burstLimitDelete"`
	RateLimitList         *int   `json:"rateLimitList"`
	BurstLimitList        *int   `json:"burstLimitList"`

	Reason string `json:"reason"` // reason for audit log
}

// GetProject gets the project info by either private or public ID.
func (s *Service) GetProject(ctx context.Context, id uuid.UUID) (*Project, api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	p, err := s.consoleDB.Projects().GetByPublicOrPrivateID(ctx, id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
			err = errs.New("project not found")
		}
		return nil, api.HTTPError{
			Status: status,
			Err:    Error.Wrap(err),
		}
	}

	u, err := s.consoleDB.Users().Get(ctx, p.OwnerID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			// If user doesn't exist, then the project doesn't exist either.
			status = http.StatusNotFound
		}
		return nil, api.HTTPError{
			Status: status,
			Err:    Error.Wrap(err),
		}
	}

	bandwidthu, storageu, segmentu, apiErr := s.getProjectUsage(ctx, p.ID)
	if apiErr.Err != nil {
		return nil, apiErr
	}

	userAgent := ""
	if p.UserAgent != nil {
		userAgent = string(p.UserAgent)
	}

	var bandwidthl *int64
	if p.BandwidthLimit != nil {
		l := p.BandwidthLimit.Int64()
		bandwidthl = &l
	}

	var storagel *int64
	if p.StorageLimit != nil {
		l := p.StorageLimit.Int64()
		storagel = &l
	}
	var userStoragel *int64
	if p.UserSpecifiedStorageLimit != nil {
		l := p.UserSpecifiedStorageLimit.Int64()
		userStoragel = &l
	}
	var userBandwidthl *int64
	if p.UserSpecifiedBandwidthLimit != nil {
		l := p.UserSpecifiedBandwidthLimit.Int64()
		userBandwidthl = &l
	}

	return &Project{
		ID:          p.ID,
		PublicID:    p.PublicID,
		Name:        p.Name,
		Description: p.Description,
		UserAgent:   userAgent,
		Owner: User{
			ID:       p.OwnerID,
			FullName: u.FullName,
			Email:    u.Email,
		},
		CreatedAt:        p.CreatedAt,
		DefaultPlacement: p.DefaultPlacement,
		RateLimit:        p.RateLimit,
		BurstLimit:       p.BurstLimit,
		RateLimitList:    p.RateLimitList,
		BurstLimitList:   p.BurstLimitList,
		RateLimitHead:    p.RateLimitHead,
		BurstLimitHead:   p.BurstLimitHead,
		RateLimitGet:     p.RateLimitGet,
		BurstLimitGet:    p.BurstLimitGet,
		RateLimitPut:     p.RateLimitPut,
		BurstLimitPut:    p.BurstLimitPut,
		RateLimitDelete:  p.RateLimitDelete,
		BurstLimitDelete: p.BurstLimitDelete,
		MaxBuckets:       p.MaxBuckets,
		ProjectUsageLimits: ProjectUsageLimits[*int64]{
			BandwidthLimit:        bandwidthl,
			UserSetBandwidthLimit: userBandwidthl,
			BandwidthUsed:         bandwidthu,
			StorageLimit:          storagel,
			UserSetStorageLimit:   userStoragel,
			StorageUsed:           storageu,
			SegmentLimit:          p.SegmentLimit,
			SegmentUsed:           segmentu,
		},
	}, api.HTTPError{}
}

// getProjectLimits returns the project's limits returning the default limits when the limits are
// `nil` in the DB projects table.
//
// id is the ID of a project (NOT the public ID).
func (s *Service) getProjectLimits(ctx context.Context, id uuid.UUID) (bandwidth, storage, segment int64, _ api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	// We return status 409 in the rare case that a project is deleted
	// before its limits can be obtained.
	makeDBErr := func(err error) api.HTTPError {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		return api.HTTPError{
			Status: status,
			Err:    Error.Wrap(err),
		}
	}

	bandwidthl, err := s.accounting.GetProjectBandwidthLimit(ctx, id)
	if err != nil {
		return 0, 0, 0, makeDBErr(err)
	}

	storagel, err := s.accounting.GetProjectStorageLimit(ctx, id)
	if err != nil {
		return 0, 0, 0, makeDBErr(err)
	}

	segmentl, err := s.accounting.GetProjectSegmentLimit(ctx, id)
	if err != nil {
		return 0, 0, 0, makeDBErr(err)
	}

	return bandwidthl.Int64(), storagel.Int64(), segmentl.Int64(), api.HTTPError{}
}

// getProjectUsage returns the project's usage and limits. If there is an error connecting to Redis
// live accounting cache, some of the usage fields are nil and it will be reported as a warning log
// message.
//
// id is the ID of a project (NOT the public ID).
func (s *Service) getProjectUsage(
	ctx context.Context,
	id uuid.UUID,
) (bandwidth int64, storage, segment *int64, _ api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	bandwidt, err := s.accounting.GetProjectBandwidthTotals(ctx, id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		return 0, nil, nil, api.HTTPError{
			Status: status,
			Err:    Error.Wrap(err),
		}
	}

	var cacheErrs []error
	handleLiveAccountingErr := func(err error) *api.HTTPError {
		if accounting.ErrSystemOrNetError.Has(err) {
			cacheErrs = append(cacheErrs, err)
			return nil
		}

		return &api.HTTPError{
			Status: http.StatusInternalServerError,
			Err:    Error.Wrap(err),
		}
	}

	// GetProjectStorageAndSegmentUsage uses the live accounting, so we need to check if
	// there is an error connecting to it.
	storageUsage, segmentUsage, err := s.accounting.GetProjectStorageAndSegmentUsage(ctx, id)
	if err != nil {
		if aerr := handleLiveAccountingErr(err); aerr != nil {
			return 0, nil, nil, *aerr
		}
	} else {
		storage = &storageUsage
		segment = &segmentUsage
	}

	if len(cacheErrs) != 0 {
		s.log.Warn(
			"Error getting project usage data from live accounting cache",
			zap.Errors("errors", cacheErrs),
		)
	}

	return bandwidt, storage, segment, api.HTTPError{}
}

// UpdateProjectLimits updates the project's max buckets, storage, bandwidth, segment, rate, and burst limits.
func (s *Service) UpdateProjectLimits(ctx context.Context, id uuid.UUID, req ProjectLimitsUpdateRequest) (*Project, api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	if req.Reason == "" {
		return nil, api.HTTPError{
			Status: http.StatusBadRequest,
			Err:    Error.New("reason is required"),
		}
	}

	p, err := s.consoleDB.Projects().GetByPublicID(ctx, id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
			err = errs.New("project not found")
		}
		return nil, api.HTTPError{
			Status: status,
			Err:    Error.Wrap(err),
		}
	}

	toUpdate, err := s.validateProjectLimitRequest(p, req)
	if err != nil {
		return nil, api.HTTPError{
			Status: http.StatusBadRequest,
			Err:    Error.Wrap(err),
		}
	}

	err = s.consoleDB.Projects().UpdateLimitsGeneric(ctx, p.ID, toUpdate)
	if err != nil {
		return nil, api.HTTPError{
			Status: http.StatusInternalServerError,
			Err:    Error.Wrap(err),
		}
	}

	return s.GetProject(ctx, id)
}

func (s *Service) validateProjectLimitRequest(p *console.Project, req ProjectLimitsUpdateRequest) (toUpdate []console.Limit, err error) {
	var errGroup errs.Group
	intTo64 := func(i *int) *int64 {
		if i == nil {
			return nil
		}
		v := int64(*i)
		return &v
	}
	sizeTo64 := func(i *memory.Size) *int64 {
		if i == nil {
			return nil
		}
		i64 := i.Int64()
		return &i64
	}

	limits := []struct {
		requestValue *int64
		currentValue *int64
		limitKind    console.LimitKind
		disallowNull bool
	}{
		{
			requestValue: req.StorageLimit,
			currentValue: sizeTo64(p.StorageLimit),
			limitKind:    console.StorageLimit,
			disallowNull: true,
		}, {
			requestValue: req.BandwidthLimit,
			currentValue: sizeTo64(p.BandwidthLimit),
			limitKind:    console.BandwidthLimit,
			disallowNull: true,
		}, {
			requestValue: req.SegmentLimit,
			currentValue: p.SegmentLimit,
			limitKind:    console.SegmentLimit,
			disallowNull: true,
		}, {
			requestValue: intTo64(req.MaxBuckets),
			currentValue: intTo64(p.MaxBuckets),
			limitKind:    console.BucketsLimit,
		}, {
			requestValue: intTo64(req.RateLimit),
			currentValue: intTo64(p.RateLimit),
			limitKind:    console.RateLimit,
		}, {
			requestValue: intTo64(req.BurstLimit),
			currentValue: intTo64(p.BurstLimit),
			limitKind:    console.BurstLimit,
		}, {
			requestValue: req.UserSetStorageLimit,
			currentValue: sizeTo64(p.UserSpecifiedStorageLimit),
			limitKind:    console.UserSetStorageLimit,
		}, {
			requestValue: req.UserSetBandwidthLimit,
			currentValue: sizeTo64(p.UserSpecifiedBandwidthLimit),
			limitKind:    console.UserSetBandwidthLimit,
		}, {
			requestValue: intTo64(req.RateLimitHead),
			currentValue: intTo64(p.RateLimitHead),
			limitKind:    console.RateLimitHead,
		}, {
			requestValue: intTo64(req.BurstLimitHead),
			currentValue: intTo64(p.BurstLimitHead),
			limitKind:    console.BurstLimitHead,
		}, {
			requestValue: intTo64(req.RateLimitGet),
			currentValue: intTo64(p.RateLimitGet),
			limitKind:    console.RateLimitGet,
		}, {
			requestValue: intTo64(req.BurstLimitGet),
			currentValue: intTo64(p.BurstLimitGet),
			limitKind:    console.BurstLimitGet,
		}, {
			requestValue: intTo64(req.RateLimitPut),
			currentValue: intTo64(p.RateLimitPut),
			limitKind:    console.RateLimitPut,
		}, {
			requestValue: intTo64(req.BurstLimitPut),
			currentValue: intTo64(p.BurstLimitPut),
			limitKind:    console.BurstLimitPut,
		}, {
			requestValue: intTo64(req.RateLimitDelete),
			currentValue: intTo64(p.RateLimitDelete),
			limitKind:    console.RateLimitDelete,
		}, {
			requestValue: intTo64(req.BurstLimitDelete),
			currentValue: intTo64(p.BurstLimitDelete),
			limitKind:    console.BurstLimitDelete,
		}, {
			requestValue: intTo64(req.RateLimitList),
			currentValue: intTo64(p.RateLimitList),
			limitKind:    console.RateLimitList,
		}, {
			requestValue: intTo64(req.BurstLimitList),
			currentValue: intTo64(p.BurstLimitList),
			limitKind:    console.BurstLimitList,
		},
	}
	for _, limit := range limits {
		if limit.requestValue == nil {
			continue
		}

		allowNull := !limit.disallowNull
		if allowNull && *limit.requestValue == NullableLimitValue && limit.currentValue != nil {
			toUpdate = append(toUpdate, console.Limit{Kind: limit.limitKind, Value: nil})
			continue
		}
		if *limit.requestValue < 0 {
			errGroup = append(errGroup, errs.New("%s cannot be negative", limit.limitKind))
			continue
		}
		if limit.currentValue == nil || *limit.requestValue != *limit.currentValue {
			toUpdate = append(toUpdate, console.Limit{Kind: limit.limitKind, Value: limit.requestValue})
		}
	}

	if err = errGroup.Err(); err != nil {
		return nil, err
	}

	return toUpdate, nil
}
