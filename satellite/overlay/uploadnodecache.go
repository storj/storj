// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"time"

	"go.uber.org/zap"

	"storj.io/common/sync2"
	"storj.io/storj/satellite/nodeselection"
)

// UploadNodeCache caches all storage nodes that qualify for uploads.
// Unlike UploadSelectionCache, it stores the raw node list without building placement-specific selectors.
type UploadNodeCache struct {
	log             *zap.Logger
	db              UploadSelectionDB
	selectionConfig NodeSelectionConfig

	cache sync2.ReadCacheOf[[]*nodeselection.SelectedNode]
}

// NewUploadNodeCache creates a new UploadNodeCache.
func NewUploadNodeCache(log *zap.Logger, db UploadSelectionDB, staleness time.Duration, config NodeSelectionConfig) (*UploadNodeCache, error) {
	c := &UploadNodeCache{
		log:             log,
		db:              db,
		selectionConfig: config,
	}
	return c, c.cache.Init(staleness/2, staleness, c.read)
}

// Run runs the background refresh loop.
func (c *UploadNodeCache) Run(ctx context.Context) (err error) {
	return c.cache.Run(ctx)
}

// Refresh forces a cache refresh. Useful for tests.
func (c *UploadNodeCache) Refresh(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	_, err = c.cache.RefreshAndGet(ctx, time.Now())
	return err
}

func (c *UploadNodeCache) read(ctx context.Context) (_ []*nodeselection.SelectedNode, err error) {
	defer mon.Task()(&ctx)(&err)

	reputable, new, err := c.db.SelectAllStorageNodesUpload(ctx, c.selectionConfig)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return append(append([]*nodeselection.SelectedNode{}, reputable...), new...), nil
}

// GetAllNodes returns all cached nodes.
func (c *UploadNodeCache) GetAllNodes(ctx context.Context) (_ []*nodeselection.SelectedNode, err error) {
	defer mon.Task()(&ctx)(&err)
	return c.cache.Get(ctx, time.Now())
}
