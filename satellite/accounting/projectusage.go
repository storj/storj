// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"
	"golang.org/x/sync/errgroup"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/common/memory"
)

var mon = monkit.Package()

const (
	// AverageDaysInMonth is how many days in a month
	AverageDaysInMonth = 30
	// ExpansionFactor is the expansion for redundancy, based on the default
	// redundancy scheme for the uplink.
	ExpansionFactor = 3
)

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
		bandwidthGetTotal, err = usage.GetProjectBandwidthTotals(ctx, projectID)
		return err
	})

	err = group.Wait()
	if err != nil {
		return false, 0, ErrProjectUsage.Wrap(err)
	}

	maxUsage := limit.Int64() * int64(ExpansionFactor)
	if bandwidthGetTotal >= maxUsage {
		return true, limit, nil
	}

	return false, limit, nil
}

// ExceedsStorageUsage returns true if the storage usage limits have been exceeded
// for a project in the past month (30 days). The usage limit is (e.g. 25GB) multiplied by the redundancy
// expansion factor, so that the uplinks have a raw limit.
// Ref: https://storjlabs.atlassian.net/browse/V3-1274
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

	maxUsage := limit.Int64() * int64(ExpansionFactor)
	if totalUsed >= maxUsage {
		return true, limit, nil
	}

	return false, limit, nil
}

// GetProjectStorageTotals returns total amount of storage used by project.
func (usage *Service) GetProjectStorageTotals(ctx context.Context, projectID uuid.UUID) (total int64, err error) {
	defer mon.Task()(&ctx, projectID)(&err)

	lastCountInline, lastCountRemote, err := usage.projectAccountingDB.GetStorageTotals(ctx, projectID)
	if err != nil {
		return 0, ErrProjectUsage.Wrap(err)
	}
	cachedTotal, err := usage.liveAccounting.GetProjectStorageUsage(ctx, projectID)
	if err != nil {
		return 0, ErrProjectUsage.Wrap(err)
	}
	return lastCountInline + lastCountRemote + cachedTotal, nil
}

// GetProjectBandwidthTotals returns total amount of allocated bandwidth used for past 30 days.
func (usage *Service) GetProjectBandwidthTotals(ctx context.Context, projectID uuid.UUID) (_ int64, err error) {
	defer mon.Task()(&ctx, projectID)(&err)

	from := time.Now().AddDate(0, 0, -AverageDaysInMonth) // past 30 days

	total, err := usage.projectAccountingDB.GetAllocatedBandwidthTotal(ctx, projectID, from)
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

// AddProjectStorageUsage lets the live accounting know that the given
// project has just added inlineSpaceUsed bytes of inline space usage
// and remoteSpaceUsed bytes of remote space usage.
func (usage *Service) AddProjectStorageUsage(ctx context.Context, projectID uuid.UUID, inlineSpaceUsed, remoteSpaceUsed int64) (err error) {
	defer mon.Task()(&ctx)(&err)
	return usage.liveAccounting.AddProjectStorageUsage(ctx, projectID, inlineSpaceUsed, remoteSpaceUsed)
}
