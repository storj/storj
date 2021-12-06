// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import "storj.io/common/memory"

// ProjectUsageLimits holds project usage limits and current usage.
type ProjectUsageLimits struct {
	StorageLimit   int64 `json:"storageLimit"`
	BandwidthLimit int64 `json:"bandwidthLimit"`
	StorageUsed    int64 `json:"storageUsed"`
	BandwidthUsed  int64 `json:"bandwidthUsed"`
	ObjectCount    int64 `json:"objectCount"`
	SegmentCount   int64 `json:"segmentCount"`
}

// UserProjectLimits holds a users storage, bandwidth, and segment limits for new projects.
type UserProjectLimits struct {
	BandwidthLimit memory.Size `json:"bandwidthLimit"`
	StorageLimit   memory.Size `json:"storageUsed"`
	SegmentLimit   int64       `json:"segmentLimit"`
}
