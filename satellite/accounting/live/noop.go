// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package live

import (
	"context"
	"time"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/accounting"
)

type noopCache struct {
}

// GetProjectStorageUsage noop method.
func (noopCache) GetProjectStorageUsage(ctx context.Context, projectID uuid.UUID) (totalUsed int64, err error) {
	return 0, nil

}

// GetProjectBandwidthUsage noop method.
func (noopCache) GetProjectBandwidthUsage(ctx context.Context, projectID uuid.UUID, now time.Time) (currentUsed int64, err error) {
	return 0, nil
}

// GetProjectStorageAndSegmentUsage noop method.
func (noopCache) GetProjectStorageAndSegmentUsage(ctx context.Context, projectID uuid.UUID) (storage, segment int64, err error) {
	return 0, 0, nil

}

// AddProjectSegmentUsageUpToLimit noop method.
func (noopCache) AddProjectSegmentUsageUpToLimit(ctx context.Context, projectID uuid.UUID, increment int64, segmentLimit int64) error {
	return nil
}

// InsertProjectBandwidthUsage noop method.
func (noopCache) InsertProjectBandwidthUsage(ctx context.Context, projectID uuid.UUID, value int64, ttl time.Duration, now time.Time) (inserted bool, _ error) {
	return true, nil
}

// UpdateProjectBandwidthUsage noop method.
func (noopCache) UpdateProjectBandwidthUsage(ctx context.Context, projectID uuid.UUID, increment int64, ttl time.Duration, now time.Time) error {
	return nil
}

// UpdateProjectSegmentUsage noop method.
func (noopCache) UpdateProjectSegmentUsage(ctx context.Context, projectID uuid.UUID, increment int64) error {
	return nil
}

// AddProjectStorageUsage noop method.
func (noopCache) AddProjectStorageUsage(ctx context.Context, projectID uuid.UUID, spaceUsed int64) error {
	return nil
}

// AddProjectStorageUsageUpToLimit noop method.
func (noopCache) AddProjectStorageUsageUpToLimit(ctx context.Context, projectID uuid.UUID, increment int64, spaceLimit int64) error {
	return nil
}

// UpdateProjectStorageAndSegmentUsage noop method.
func (noopCache) UpdateProjectStorageAndSegmentUsage(ctx context.Context, projectID uuid.UUID, storageIncrement, segmentIncrement int64) (err error) {
	return nil
}

// GetAllProjectTotals noop method.
func (noopCache) GetAllProjectTotals(ctx context.Context) (map[uuid.UUID]accounting.Usage, error) {
	return map[uuid.UUID]accounting.Usage{}, nil
}

// Close noop method.
func (noopCache) Close() error {
	return nil
}
