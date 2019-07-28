// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"
	"golang.org/x/sync/errgroup"

	"storj.io/storj/internal/memory"
	"storj.io/storj/satellite/accounting/live"
)

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

// ProjectUsage defines project usage
type ProjectUsage struct {
	projectAccountingDB ProjectAccounting
	liveAccounting      live.Service
	maxAlphaUsage       memory.Size
}

// NewProjectUsage created new instance of project usage service
func NewProjectUsage(projectAccountingDB ProjectAccounting, liveAccounting live.Service, maxAlphaUsage memory.Size) *ProjectUsage {
	return &ProjectUsage{
		projectAccountingDB: projectAccountingDB,
		liveAccounting:      liveAccounting,
		maxAlphaUsage:       maxAlphaUsage,
	}
}

// ExceedsBandwidthUsage returns true if the bandwidth usage limits have been exceeded
// for a project in the past month (30 days). The usage limit is (e.g 25GB) multiplied by the redundancy
// expansion factor, so that the uplinks have a raw limit.
// Ref: https://storjlabs.atlassian.net/browse/V3-1274
func (usage *ProjectUsage) ExceedsBandwidthUsage(ctx context.Context, projectID uuid.UUID, bucketID []byte) (_ bool, limit memory.Size, err error) {
	defer mon.Task()(&ctx)(&err)

	var group errgroup.Group
	var bandwidthGetTotal int64
	limit = usage.maxAlphaUsage

	// TODO(michal): to reduce db load, consider using a cache to retrieve the project.UsageLimit value if needed
	group.Go(func() error {
		projectLimit, err := usage.projectAccountingDB.GetProjectUsageLimits(ctx, projectID)
		if projectLimit > 0 {
			limit = projectLimit
		}
		return err
	})
	group.Go(func() error {
		var err error
		from := time.Now().AddDate(0, 0, -AverageDaysInMonth) // past 30 days
		bandwidthGetTotal, err = usage.projectAccountingDB.GetAllocatedBandwidthTotal(ctx, projectID, from)
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
func (usage *ProjectUsage) ExceedsStorageUsage(ctx context.Context, projectID uuid.UUID) (_ bool, limit memory.Size, err error) {
	defer mon.Task()(&ctx)(&err)

	var group errgroup.Group
	var inlineTotal, remoteTotal int64
	limit = usage.maxAlphaUsage

	// TODO(michal): to reduce db load, consider using a cache to retrieve the project.UsageLimit value if needed
	group.Go(func() error {
		projectLimit, err := usage.projectAccountingDB.GetProjectUsageLimits(ctx, projectID)
		if projectLimit > 0 {
			limit = projectLimit
		}
		return err
	})
	group.Go(func() error {
		var err error
		inlineTotal, remoteTotal, err = usage.getProjectStorageTotals(ctx, projectID)
		return err
	})
	err = group.Wait()
	if err != nil {
		return false, 0, ErrProjectUsage.Wrap(err)
	}

	maxUsage := limit.Int64() * int64(ExpansionFactor)
	if inlineTotal+remoteTotal >= maxUsage {
		return true, limit, nil
	}

	return false, limit, nil
}

func (usage *ProjectUsage) getProjectStorageTotals(ctx context.Context, projectID uuid.UUID) (inline int64, remote int64, err error) {
	defer mon.Task()(&ctx)(&err)

	lastCountInline, lastCountRemote, err := usage.projectAccountingDB.GetStorageTotals(ctx, projectID)
	if err != nil {
		return 0, 0, err
	}
	rtInline, rtRemote, err := usage.liveAccounting.GetProjectStorageUsage(ctx, projectID)
	if err != nil {
		return 0, 0, err
	}
	return lastCountInline + rtInline, lastCountRemote + rtRemote, nil
}

// AddProjectStorageUsage lets the live accounting know that the given
// project has just added inlineSpaceUsed bytes of inline space usage
// and remoteSpaceUsed bytes of remote space usage.
func (usage *ProjectUsage) AddProjectStorageUsage(ctx context.Context, projectID uuid.UUID, inlineSpaceUsed, remoteSpaceUsed int64) (err error) {
	defer mon.Task()(&ctx)(&err)
	return usage.liveAccounting.AddProjectStorageUsage(ctx, projectID, inlineSpaceUsed, remoteSpaceUsed)
}
