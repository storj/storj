// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting

import (
	"context"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"golang.org/x/sync/errgroup"

	"storj.io/common/memory"
	"storj.io/common/uuid"
)

var mon = monkit.Package()

var (
	// ErrProjectUsage general error for project usage.
	ErrProjectUsage = errs.Class("project usage error")
)

// Service is handling project usage related logic.
//
// architecture: Service
type Service struct {
	projectAccountingDB ProjectAccounting
	liveAccounting      Cache
	projectLimitCache   *ProjectLimitCache
	bandwidthCacheTTL   time.Duration
	nowFn               func() time.Time
}

// NewService created new instance of project usage service.
func NewService(projectAccountingDB ProjectAccounting, liveAccounting Cache, limitCache *ProjectLimitCache, bandwidthCacheTTL time.Duration) *Service {
	return &Service{
		projectAccountingDB: projectAccountingDB,
		liveAccounting:      liveAccounting,
		projectLimitCache:   limitCache,
		bandwidthCacheTTL:   bandwidthCacheTTL,
		nowFn:               time.Now,
	}
}

// ExceedsBandwidthUsage returns true if the bandwidth usage limits have been exceeded
// for a project in the past month (30 days). The usage limit is (e.g 25GB) multiplied by the redundancy
// expansion factor, so that the uplinks have a raw limit.
//
// Among others,it can return one of the following errors returned by
// storj.io/storj/satellite/accounting.Cache except the ErrKeyNotFound, wrapped
// by ErrProjectUsage.
func (usage *Service) ExceedsBandwidthUsage(ctx context.Context, projectID uuid.UUID) (_ bool, limit memory.Size, err error) {
	defer mon.Task()(&ctx)(&err)

	var group errgroup.Group
	var bandwidthGetTotal int64
	var bandwidthUsage int64

	group.Go(func() error {
		var err error
		limit, err = usage.projectLimitCache.GetProjectBandwidthLimit(ctx, projectID)
		return err
	})
	group.Go(func() error {
		var err error

		// Get the current bandwidth usage from cache.
		bandwidthUsage, err = usage.liveAccounting.GetProjectBandwidthUsage(ctx, projectID, usage.nowFn())
		if err != nil {
			// Verify If the cache key was not found
			if ErrKeyNotFound.Has(err) {

				// Get current bandwidth value from database.
				now := usage.nowFn()
				bandwidthGetTotal, err = usage.GetProjectAllocatedBandwidth(ctx, projectID, now.Year(), now.Month())
				if err != nil {
					return err
				}

				// Create cache key with database value.
				err = usage.liveAccounting.UpdateProjectBandwidthUsage(ctx, projectID, bandwidthGetTotal, usage.bandwidthCacheTTL, usage.nowFn())
				if err != nil {
					return err
				}

				bandwidthUsage = bandwidthGetTotal
			}
		}
		return err
	})

	err = group.Wait()
	if err != nil {
		return false, 0, ErrProjectUsage.Wrap(err)
	}

	// Verify the bandwidth usage cache.
	if bandwidthUsage >= limit.Int64() {
		return true, limit, nil
	}

	return false, limit, nil
}

// ExceedsStorageUsage returns true if the storage usage for a project is currently over that project's limit.
func (usage *Service) ExceedsStorageUsage(ctx context.Context, projectID uuid.UUID) (_ bool, limit memory.Size, err error) {
	defer mon.Task()(&ctx)(&err)

	var group errgroup.Group
	var totalUsed int64

	group.Go(func() error {
		var err error
		limit, err = usage.projectLimitCache.GetProjectStorageLimit(ctx, projectID)
		return err
	})
	group.Go(func() error {
		var err error
		totalUsed, err = usage.GetProjectStorageTotals(ctx, projectID)
		return err
	})

	err = group.Wait()
	if err != nil {
		return false, 0, ErrProjectUsage.Wrap(err)
	}

	if totalUsed >= limit.Int64() {
		return true, limit, nil
	}

	return false, limit, nil
}

// GetProjectStorageTotals returns total amount of storage used by project.
//
// It can return one of the following errors returned by
// storj.io/storj/satellite/accounting.Cache.GetProjectStorageUsage except the
// ErrKeyNotFound, wrapped by ErrProjectUsage.
func (usage *Service) GetProjectStorageTotals(ctx context.Context, projectID uuid.UUID) (total int64, err error) {
	defer mon.Task()(&ctx, projectID)(&err)

	total, err = usage.liveAccounting.GetProjectStorageUsage(ctx, projectID)
	if ErrKeyNotFound.Has(err) {
		return 0, nil
	}

	return total, ErrProjectUsage.Wrap(err)
}

// GetProjectBandwidthTotals returns total amount of allocated bandwidth used for past 30 days.
func (usage *Service) GetProjectBandwidthTotals(ctx context.Context, projectID uuid.UUID) (_ int64, err error) {
	defer mon.Task()(&ctx, projectID)(&err)

	// from the beginning of the current month
	year, month, _ := usage.nowFn().Date()
	from := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)

	total, err := usage.projectAccountingDB.GetAllocatedBandwidthTotal(ctx, projectID, from)
	return total, ErrProjectUsage.Wrap(err)
}

// GetProjectAllocatedBandwidth returns project allocated bandwidth for the specified year and month.
func (usage *Service) GetProjectAllocatedBandwidth(ctx context.Context, projectID uuid.UUID, year int, month time.Month) (_ int64, err error) {
	defer mon.Task()(&ctx, projectID)(&err)

	total, err := usage.projectAccountingDB.GetProjectAllocatedBandwidth(ctx, projectID, year, month)
	return total, ErrProjectUsage.Wrap(err)
}

// GetProjectStorageLimit returns current project storage limit.
func (usage *Service) GetProjectStorageLimit(ctx context.Context, projectID uuid.UUID) (_ memory.Size, err error) {
	defer mon.Task()(&ctx, projectID)(&err)
	return usage.projectLimitCache.GetProjectStorageLimit(ctx, projectID)
}

// GetProjectBandwidthLimit returns current project bandwidth limit.
func (usage *Service) GetProjectBandwidthLimit(ctx context.Context, projectID uuid.UUID) (_ memory.Size, err error) {
	defer mon.Task()(&ctx, projectID)(&err)
	return usage.projectLimitCache.GetProjectBandwidthLimit(ctx, projectID)
}

// UpdateProjectLimits sets new value for project's bandwidth and storage limit.
func (usage *Service) UpdateProjectLimits(ctx context.Context, projectID uuid.UUID, limit memory.Size) (err error) {
	defer mon.Task()(&ctx, projectID)(&err)

	return ErrProjectUsage.Wrap(usage.projectAccountingDB.UpdateProjectUsageLimit(ctx, projectID, limit))
}

// GetProjectBandwidthUsage get the current bandwidth usage from cache.
//
// It can return one of the following errors returned by
// storj.io/storj/satellite/accounting.Cache.GetProjectBandwidthUsage, wrapped
// by ErrProjectUsage.
func (usage *Service) GetProjectBandwidthUsage(ctx context.Context, projectID uuid.UUID) (currentUsed int64, err error) {
	return usage.liveAccounting.GetProjectBandwidthUsage(ctx, projectID, usage.nowFn())
}

// UpdateProjectBandwidthUsage increments the bandwidth cache key for a specific project.
//
// It can return one of the following errors returned by
// storj.io/storj/satellite/accounting.Cache.UpdatProjectBandwidthUsage, wrapped
// by ErrProjectUsage.
func (usage *Service) UpdateProjectBandwidthUsage(ctx context.Context, projectID uuid.UUID, increment int64) (err error) {
	return usage.liveAccounting.UpdateProjectBandwidthUsage(ctx, projectID, increment, usage.bandwidthCacheTTL, usage.nowFn())
}

// AddProjectStorageUsage lets the live accounting know that the given
// project has just added spaceUsed bytes of storage (from the user's
// perspective; i.e. segment size).
//
// It can return one of the following errors returned by
// storj.io/storj/satellite/accounting.Cache.AddProjectStorageUsage, wrapped by
// ErrProjectUsage.
func (usage *Service) AddProjectStorageUsage(ctx context.Context, projectID uuid.UUID, spaceUsed int64) (err error) {
	defer mon.Task()(&ctx, projectID)(&err)
	return usage.liveAccounting.AddProjectStorageUsage(ctx, projectID, spaceUsed)
}

// SetNow allows tests to have the Service act as if the current time is whatever they want.
func (usage *Service) SetNow(now func() time.Time) {
	usage.nowFn = now
}
