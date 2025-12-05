// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package filestore

import (
	"context"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"storj.io/storj/storagenode/blobstore"
)

const infoMaxAge = time.Minute

type infoAge struct {
	info blobstore.DiskInfo
	age  time.Time
}

// DirSpaceInfo is a helper to get disk space information for a directory.
type DirSpaceInfo struct {
	path string

	mu   sync.Mutex
	info atomic.Pointer[infoAge]
}

// NewDirSpaceInfo creates a new DirSpaceInfo.
func NewDirSpaceInfo(path string) *DirSpaceInfo {
	return &DirSpaceInfo{
		path: path,
	}
}

// AvailableSpace returns the available space for the cache directory.
func (c *DirSpaceInfo) AvailableSpace(ctx context.Context) (blobstore.DiskInfo, error) {
	if info := c.info.Load(); info != nil && time.Since(info.age) < infoMaxAge {
		return info.info, nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	if info := c.info.Load(); info != nil && time.Since(info.age) < infoMaxAge {
		return info.info, nil
	}

	path, err := filepath.Abs(c.path)
	if err != nil {
		return blobstore.DiskInfo{}, err
	}
	info, err := DiskInfoFromPath(path)
	if err != nil {
		return blobstore.DiskInfo{}, err
	}
	c.info.Store(&infoAge{info: info, age: time.Now()})
	return info, nil
}
