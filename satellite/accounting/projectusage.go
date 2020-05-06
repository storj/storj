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
	// ErrProjectUsage general error for project usage
	ErrProjectUsage = errs.Class("project usage error")
)

// Service is handling project usage related logic.
//
// architecture: Service
type Service struct {
	projectAccountingDB ProjectAccounting
	liveAccounting      Cache
	maxAlphaUsage       memory.Size
}

// NewService created new instance of project usage service.
func NewService(projectAccountingDB ProjectAccounting, liveAccounting Cache, maxAlphaUsage memory.Size) *Service {
	return &Service{
		projectAccountingDB: projectAccountingDB,
		liveAccounting:      liveAccounting,
		maxAlphaUsage:       maxAlphaUsage,
	}
}

// ExceedsBandwidthUsage returns true if the bandwidth usage limits have been exceeded
// for a project in the past month (30 days). The usage limit is (e.g 25GB) multiplied by the redundancy
// expansion factor, so that the uplinks have a raw limit.
// Ref: https://storjlabs.atlassian.net/browse/V3-1274
func (usage *Service) ExceedsBandwidthUsage(ctx context.Context, projectID uuid.UUID, bucketID []byte) (_ bool, limit memory.Size, err error) {
	defer mon.Task()(&ctx)(&err)

	var group errgroup.Group
	var bandwidthGetTotal int64

	// TODO(michal): to reduce db load, consider using a cache to retrieve the project.UsageLimit value if needed
	group.Go(func() error {
		var err error
		limit, err = usage.GetProjectBandwidthLimit(ctx, projectID)
		return err
	})
	group.Go(func() error {
		var err error
		bandwidthGetTotal, err = usage.GetCurrentBandwidthAllocated(ctx, projectID)
		return err
	})

	err = group.Wait()
	if err != nil {
		return false, 0, ErrProjectUsage.Wrap(err)
	}

	if bandwidthGetTotal >= limit.Int64() {
		return true, limit, nil
	}

	return false, limit, nil
}

// ExceedsStorageUsage returns true if the storage usage for a project is currently over that project's limit.
func (usage *Service) ExceedsStorageUsage(ctx context.Context, projectID uuid.UUID) (_ bool, limit memory.Size, err error) {
	defer mon.Task()(&ctx)(&err)

	var group errgroup.Group
	var totalUsed int64

	// TODO(michal): to reduce db load, consider using a cache to retrieve the project.UsageLimit value if needed
	group.Go(func() error {
		var err error
		limit, err = usage.GetProjectStorageLimit(ctx, projectID)
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
func (usage *Service) GetProjectStorageTotals(ctx context.Context, projectID uuid.UUID) (total int64, err error) {
	defer mon.Task()(&ctx, projectID)(&err)

	total, err = usage.liveAccounting.GetProjectStorageUsage(ctx, projectID)

	return total, ErrProjectUsage.Wrap(err)
}

// GetProjectBandwidthTotals returns total amount of allocated bandwidth used for past 30 days.
func (usage *Service) GetProjectBandwidthTotals(ctx context.Context, projectID uuid.UUID) (_ int64, err error) {
	defer mon.Task()(&ctx, projectID)(&err)

	// from the beginning of the current month
	year, month, _ := time.Now().Date()
	from := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)

	total, err := usage.projectAccountingDB.GetAllocatedBandwidthTotal(ctx, projectID, from)
	return total, ErrProjectUsage.Wrap(err)
}

// GetCurrentBandwidthAllocated returns allocated bandwidth for the current month
func (usage *Service) GetCurrentBandwidthAllocated(ctx context.Context, projectID uuid.UUID) (_ int64, err error) {
	defer mon.Task()(&ctx, projectID)(&err)

	total, err := usage.projectAccountingDB.GetCurrentBandwidthAllocated(ctx, projectID)
	return total, ErrProjectUsage.Wrap(err)
}

// GetProjectStorageLimit returns current project storage limit.
func (usage *Service) GetProjectStorageLimit(ctx context.Context, projectID uuid.UUID) (_ memory.Size, err error) {
	defer mon.Task()(&ctx, projectID)(&err)

	limit, err := usage.projectAccountingDB.GetProjectStorageLimit(ctx, projectID)
	if err != nil {
		return 0, ErrProjectUsage.Wrap(err)
	}
	if limit == 0 {
		return usage.maxAlphaUsage, nil
	}

	return limit, nil
}

// GetProjectBandwidthLimit returns current project bandwidth limit.
func (usage *Service) GetProjectBandwidthLimit(ctx context.Context, projectID uuid.UUID) (_ memory.Size, err error) {
	defer mon.Task()(&ctx, projectID)(&err)

	limit, err := usage.projectAccountingDB.GetProjectBandwidthLimit(ctx, projectID)
	if err != nil {
		return 0, ErrProjectUsage.Wrap(err)
	}
	if limit == 0 {
		return usage.maxAlphaUsage, nil
	}

	return limit, nil
}

// UpdateProjectLimits sets new value for project's bandwidth and storage limit.
func (usage *Service) UpdateProjectLimits(ctx context.Context, projectID uuid.UUID, limit memory.Size) (err error) {
	defer mon.Task()(&ctx, projectID)(&err)

	return ErrProjectUsage.Wrap(usage.projectAccountingDB.UpdateProjectUsageLimit(ctx, projectID, limit))
}

// AddProjectStorageUsage lets the live accounting know that the given
// project has just added spaceUsed bytes of storage (from the user's
// perspective; i.e. segment size).
func (usage *Service) AddProjectStorageUsage(ctx context.Context, projectID uuid.UUID, spaceUsed int64) (err error) {
	defer mon.Task()(&ctx, projectID)(&err)
	return usage.liveAccounting.AddProjectStorageUsage(ctx, projectID, spaceUsed)
}
