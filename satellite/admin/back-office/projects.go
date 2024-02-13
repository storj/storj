// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package admin

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/private/api"
	"storj.io/storj/satellite/accounting"
)

// Project contains the information and configurations of a project.
type Project struct {
	ID               uuid.UUID                 `json:"id"` // This is the public ID
	Name             string                    `json:"name"`
	Description      string                    `json:"description"`
	UserAgent        string                    `json:"userAgent"`
	Owner            User                      `json:"owner"`
	CreatedAt        time.Time                 `json:"createdAt"`
	DefaultPlacement storj.PlacementConstraint `json:"defaultPlacement"`

	// RateLimit is `nil` when satellite applies the configured default rate limit.
	RateLimit *int `json:"rateLimit"`
	// BurstLimit is `nil` when satellite applies the configured default burst limit.
	BurstLimit *int `json:"burstLimit"`
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
	BandwidthLimit T      `json:"bandwidthLimit"`
	BandwidthUsed  int64  `json:"bandwidthUsed"`
	StorageLimit   T      `json:"storageLimit"`
	StorageUsed    *int64 `json:"storageUsed"`
	SegmentLimit   T      `json:"segmentLimit"`
	SegmentUsed    *int64 `json:"segmentUsed"`
}

// ProjectLimitsUpdate contains all limit values to be updated.
type ProjectLimitsUpdate struct {
	MaxBuckets     int   `json:"maxBuckets"`
	StorageLimit   int64 `json:"storageLimit"`
	BandwidthLimit int64 `json:"bandwidthLimit"`
	SegmentLimit   int64 `json:"segmentLimit"`
	RateLimit      int   `json:"rateLimit"`
	BurstLimit     int   `json:"burstLimit"`
}

// GetProject gets the project info.
func (s *Service) GetProject(ctx context.Context, id uuid.UUID) (*Project, api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	p, err := s.consoleDB.Projects().GetByPublicID(ctx, id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
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

	maxBuckets := &s.defaults.MaxBuckets
	if p.MaxBuckets != nil {
		*maxBuckets = *p.MaxBuckets
	}

	rate := &s.defaults.RateLimit
	if p.RateLimit != nil {
		rate = p.RateLimit
	}

	burst := &s.defaults.RateLimit
	if p.BurstLimit != nil {
		burst = p.BurstLimit
	}

	return &Project{
		ID:          p.PublicID,
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
		RateLimit:        rate,
		BurstLimit:       burst,
		MaxBuckets:       maxBuckets,
		ProjectUsageLimits: ProjectUsageLimits[*int64]{
			BandwidthLimit: bandwidthl,
			BandwidthUsed:  bandwidthu,
			StorageLimit:   storagel,
			StorageUsed:    storageu,
			SegmentLimit:   p.SegmentLimit,
			SegmentUsed:    segmentu,
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

	// GetProjectStorageTotals uses the live accounting, so we need to check if
	// there is an error connecting to it.
	storageUsage, err := s.accounting.GetProjectStorageTotals(ctx, id)
	if err != nil {
		if aerr := handleLiveAccountingErr(err); aerr != nil {
			return 0, nil, nil, *aerr
		}
	} else {
		storage = &storageUsage
	}

	// GetProjectSegmentTotals uses the live accounting, so we need to check if
	// there is an error connecting to it.
	segmentUsage, err := s.accounting.GetProjectSegmentTotals(ctx, id)
	if err != nil {
		if aerr := handleLiveAccountingErr(err); aerr != nil {
			return 0, nil, nil, *aerr
		}
	} else {
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
func (s *Service) UpdateProjectLimits(ctx context.Context, id uuid.UUID, req ProjectLimitsUpdate) api.HTTPError {
	var err error
	defer mon.Task()(&ctx)(&err)

	p, err := s.consoleDB.Projects().GetByPublicID(ctx, id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		return api.HTTPError{
			Status: status,
			Err:    Error.Wrap(err),
		}
	}

	buckets := &req.MaxBuckets
	if req.MaxBuckets == s.defaults.MaxBuckets {
		buckets = nil
	}
	rate := &req.RateLimit
	if req.RateLimit == s.defaults.RateLimit {
		rate = nil
	}
	burst := &req.BurstLimit
	if req.BurstLimit == s.defaults.RateLimit {
		burst = nil
	}

	// Note: usage_limit (storage), bandwidth_limit, and segment_limit columns are also nullable, but are never actually null in production.

	err = s.consoleDB.Projects().UpdateAllLimits(ctx, p.ID, &req.StorageLimit, &req.BandwidthLimit, &req.SegmentLimit, buckets, rate, burst)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		return api.HTTPError{
			Status: status,
			Err:    Error.Wrap(err),
		}
	}

	return api.HTTPError{}
}
