// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

// ProjectUsageLimits holds project usage limits and current usage.
type ProjectUsageLimits struct {
	StorageLimit   int64 `json:"storageLimit"`
	BandwidthLimit int64 `json:"bandwidthLimit"`
	StorageUsed    int64 `json:"storageUsed"`
	BandwidthUsed  int64 `json:"bandwidthUsed"`
	ObjectCount    int64 `json:"objectCount"`
	SegmentCount   int64 `json:"segmentCount"`
	RateLimit      int64 `json:"rateLimit"`
	SegmentLimit   int64 `json:"segmentLimit"`
	RateUsed       int64 `json:"rateUsed"`
	SegmentUsed    int64 `json:"segmentUsed"`
	BucketsUsed    int64 `json:"bucketsUsed"`
	BucketsLimit   int64 `json:"bucketsLimit"`
}

// UsageLimits represents storage, bandwidth, and segment limits imposed on an entity.
type UsageLimits struct {
	Storage   int64 `json:"storage"`
	Bandwidth int64 `json:"bandwidth"`
	Segment   int64 `json:"segment"`
}
