// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting

import (
	"context"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/lrucache"
	"storj.io/common/memory"
	"storj.io/common/uuid"
)

var (
	// ErrProjectLimitType error for project limit type.
	ErrProjectLimitType = errs.Class("project limit type")
	// ErrGetProjectLimit error for getting project limits from database.
	ErrGetProjectLimit = errs.Class("get project limits")
	// ErrGetProjectLimitCache error for getting project limits from cache.
	ErrGetProjectLimitCache = errs.Class("get project limits from cache")
)

// ProjectLimitDB stores information about projects limits for storage and bandwidth limits.
//
// architecture: Database
type ProjectLimitDB interface {
	// GetProjectLimits returns current project limit for both storage and bandwidth.
	GetProjectLimits(ctx context.Context, projectID uuid.UUID) (ProjectLimits, error)
}

// ProjectLimitConfig is a configuration struct for project limit.
type ProjectLimitConfig struct {
	CacheCapacity   int           `help:"number of projects to cache." releaseDefault:"20000" devDefault:"100"`
	CacheExpiration time.Duration `help:"how long to cache the project limits." releaseDefault:"10m" devDefault:"30s"`
}

// ProjectLimitCache stores the values for both storage usage limit and bandwidth limit for
// each project ID if they differ from the default limits.
type ProjectLimitCache struct {
	projectLimitDB      ProjectLimitDB
	defaultMaxUsage     memory.Size
	defaultMaxBandwidth memory.Size
	defaultMaxSegments  int64

	state *lrucache.ExpiringLRUOf[ProjectLimits]
}

// NewProjectLimitCache creates a new project limit cache to store the project limits for each project ID.
func NewProjectLimitCache(db ProjectLimitDB, defaultMaxUsage, defaultMaxBandwidth memory.Size, defaultMaxSegments int64, config ProjectLimitConfig) *ProjectLimitCache {
	return &ProjectLimitCache{
		projectLimitDB:      db,
		defaultMaxUsage:     defaultMaxUsage,
		defaultMaxBandwidth: defaultMaxBandwidth,
		defaultMaxSegments:  defaultMaxSegments,
		state: lrucache.NewOf[ProjectLimits](lrucache.Options{
			Capacity:   config.CacheCapacity,
			Expiration: config.CacheExpiration,
			Name:       "accounting-projectlimit",
		}),
	}
}

// GetLimits returns the project limits from cache.
func (c *ProjectLimitCache) GetLimits(ctx context.Context, projectID uuid.UUID) (ProjectLimits, error) {
	limits, err := c.state.Get(ctx, projectID.String(),
		func() (ProjectLimits, error) {
			return c.getProjectLimits(ctx, projectID)
		})
	if err != nil {
		return ProjectLimits{}, ErrGetProjectLimitCache.Wrap(err)
	}
	return limits, nil
}

// GetBandwidthLimit return the bandwidth usage limit for a project ID.
func (c *ProjectLimitCache) GetBandwidthLimit(ctx context.Context, projectID uuid.UUID) (_ memory.Size, err error) {
	defer mon.Task()(&ctx)(&err)
	projectLimits, err := c.GetLimits(ctx, projectID)
	if err != nil {
		return 0, err
	}
	if projectLimits.Bandwidth == nil {
		return c.defaultMaxBandwidth, nil
	}
	return memory.Size(*projectLimits.Bandwidth), nil
}

// GetSegmentLimit return the segment limit for a project ID.
func (c *ProjectLimitCache) GetSegmentLimit(ctx context.Context, projectID uuid.UUID) (_ memory.Size, err error) {
	defer mon.Task()(&ctx)(&err)
	projectLimits, err := c.GetLimits(ctx, projectID)
	if err != nil {
		return 0, err
	}
	if projectLimits.Segments == nil {
		return memory.Size(c.defaultMaxSegments), nil
	}
	return memory.Size(*projectLimits.Segments), nil
}

// getProjectLimits returns project limits from DB.
func (c *ProjectLimitCache) getProjectLimits(ctx context.Context, projectID uuid.UUID) (_ ProjectLimits, err error) {
	defer mon.Task()(&ctx, projectID)(&err)

	projectLimits, err := c.projectLimitDB.GetProjectLimits(ctx, projectID)
	if err != nil {
		return ProjectLimits{}, ErrGetProjectLimit.Wrap(err)
	}
	if projectLimits.Bandwidth == nil {
		defaultBandwidth := c.defaultMaxBandwidth.Int64()
		projectLimits.Bandwidth = &defaultBandwidth
	}
	if projectLimits.Usage == nil {
		defaultUsage := c.defaultMaxUsage.Int64()
		projectLimits.Usage = &defaultUsage
	}
	if projectLimits.Segments == nil {
		defaultSegments := c.defaultMaxSegments
		projectLimits.Segments = &defaultSegments
	}
	if projectLimits.Segments == nil {
		defaultSegments := c.defaultMaxSegments
		projectLimits.Segments = &defaultSegments
	}

	return projectLimits, nil
}
