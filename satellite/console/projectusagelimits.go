// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

// ProjectUsageLimits holds project usage limits and current usage.
type ProjectUsageLimits struct {
	StorageLimit   int64 `json:"storageLimit"`
	BandwidthLimit int64 `json:"bandwidthLimit"`
	StorageUsed    int64 `json:"storageUsed"`
	BandwidthUsed  int64 `json:"bandwidthUsed"`
}
