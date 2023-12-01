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

// User holds information about a user account.
type User struct {
	ID                 uuid.UUID                 `json:"id"`
	FullName           string                    `json:"fullName"`
	Email              string                    `json:"email"`
	PaidTier           bool                      `json:"paidTier"`
	CreatedAt          time.Time                 `json:"createdAt"`
	Status             string                    `json:"status"`
	UserAgent          string                    `json:"userAgent"`
	DefaultPlacement   storj.PlacementConstraint `json:"defaultPlacement"`
	ProjectUsageLimits []ProjectUsageLimits      `json:"projectUsageLimits"`
}

// ProjectUsageLimits holds project usage limits and current usage.
// StorageUsed, BandwidthUsed, and SegmentUsed are nil if there was
// an error connecting to the Redis live accounting cache.
type ProjectUsageLimits struct {
	ID             uuid.UUID `json:"id"` // This is the public ID
	Name           string    `json:"name"`
	StorageLimit   int64     `json:"storageLimit"`
	StorageUsed    *int64    `json:"storageUsed"`
	BandwidthLimit int64     `json:"bandwidthLimit"`
	BandwidthUsed  int64     `json:"bandwidthUsed"`
	SegmentLimit   int64     `json:"segmentLimit"`
	SegmentUsed    *int64    `json:"segmentUsed"`
}

// GetUserByEmail returns a verified user by its email address.
func (s *Service) GetUserByEmail(ctx context.Context, email string) (*User, api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	user, err := s.consoleDB.Users().GetByEmail(ctx, email)
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

	projects, err := s.consoleDB.Projects().GetOwn(ctx, user.ID)
	if err != nil {
		return nil, api.HTTPError{
			Status: http.StatusInternalServerError,
			Err:    Error.Wrap(err),
		}
	}

	// We return status 409 in the rare case that a project is deleted
	// before its limits can be obtained.
	makeDBErr := func(err error) api.HTTPError {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusConflict
		}
		return api.HTTPError{
			Status: status,
			Err:    Error.Wrap(err),
		}
	}

	var cacheErrs []error
	usageLimits := make([]ProjectUsageLimits, 0, len(projects))
	for _, project := range projects {
		usage := ProjectUsageLimits{
			ID:   project.PublicID,
			Name: project.Name,
		}

		storageLimit, err := s.accounting.GetProjectStorageLimit(ctx, project.ID)
		if err != nil {
			return nil, makeDBErr(err)
		}
		usage.StorageLimit = storageLimit.Int64()

		bandwidthLimit, err := s.accounting.GetProjectBandwidthLimit(ctx, project.ID)
		if err != nil {
			return nil, makeDBErr(err)
		}
		usage.BandwidthLimit = bandwidthLimit.Int64()

		segmentLimit, err := s.accounting.GetProjectSegmentLimit(ctx, project.ID)
		if err != nil {
			return nil, makeDBErr(err)
		}
		usage.SegmentLimit = segmentLimit.Int64()

		storageUsed, err := s.accounting.GetProjectStorageTotals(ctx, project.ID)
		if err == nil {
			usage.StorageUsed = &storageUsed
		} else if accounting.ErrSystemOrNetError.Has(err) {
			cacheErrs = append(cacheErrs, err)
		} else {
			return nil, api.HTTPError{
				Status: http.StatusInternalServerError,
				Err:    Error.Wrap(err),
			}
		}

		usage.BandwidthUsed, err = s.accounting.GetProjectBandwidthTotals(ctx, project.ID)
		if err != nil {
			return nil, makeDBErr(err)
		}

		segmentUsed, err := s.accounting.GetProjectSegmentTotals(ctx, project.ID)
		if err == nil {
			usage.SegmentUsed = &segmentUsed
		} else if accounting.ErrSystemOrNetError.Has(err) {
			cacheErrs = append(cacheErrs, err)
		} else {
			return nil, api.HTTPError{
				Status: http.StatusInternalServerError,
				Err:    Error.Wrap(err),
			}
		}

		usageLimits = append(usageLimits, usage)
	}

	if len(cacheErrs) != 0 {
		s.log.Warn("Error getting project usage data from live accounting cache", zap.Errors("errors", cacheErrs))
	}

	return &User{
		ID:                 user.ID,
		FullName:           user.FullName,
		Email:              user.Email,
		PaidTier:           user.PaidTier,
		CreatedAt:          user.CreatedAt,
		Status:             user.Status.String(),
		UserAgent:          string(user.UserAgent),
		DefaultPlacement:   user.DefaultPlacement,
		ProjectUsageLimits: usageLimits,
	}, api.HTTPError{}
}
