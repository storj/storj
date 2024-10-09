// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

// LimitKind is a generic enum type for kinds of limits.
type LimitKind int

const (
	// StorageLimit is the amount of data that can be stored in a project.
	StorageLimit LimitKind = 0
	// BandwidthLimit is the amount of egress usage allowed for a project.
	BandwidthLimit LimitKind = 1
	// UserSetStorageLimit is an optional custom storage limit that the user can specify.
	UserSetStorageLimit LimitKind = 2
	// UserSetBandwidthLimit is an optional custom egress limit that the user can specify.
	UserSetBandwidthLimit LimitKind = 3
	// SegmentLimit is the number of segments allowed in a project.
	SegmentLimit LimitKind = 4
	// BucketsLimit is the number of buckets allowed in a project.
	BucketsLimit LimitKind = 5
	// RateLimit is the catch-all rate of requests allowed in a project.
	RateLimit LimitKind = 6
	// BurstLimit is the catch-all "rate limit burst" for requests in a project.
	BurstLimit LimitKind = 7
	// RateLimitHead overrides RateLimit for "head" requests.
	RateLimitHead LimitKind = 8
	// BurstLimitHead overrides BurstLimit for "head" requests.
	BurstLimitHead LimitKind = 9
	// RateLimitGet overrides RateLimit for "get" requests.
	RateLimitGet LimitKind = 10
	// BurstLimitGet overrides BurstLimit for "get" requests.
	BurstLimitGet LimitKind = 11
	// RateLimitPut overrides RateLimit for "put" requests.
	RateLimitPut LimitKind = 12
	// BurstLimitPut overrides BurstLimit for "put" requests.
	BurstLimitPut LimitKind = 13
	// RateLimitList overrides RateLimit for "list" requests.
	RateLimitList LimitKind = 14
	// BurstLimitList overrides BurstLimit for "list" requests.
	BurstLimitList LimitKind = 15
	// RateLimitDelete overrides RateLimit for "delete" requests.
	RateLimitDelete LimitKind = 16
	// BurstLimitDelete overrides BurstLimit for "delete" requests.
	BurstLimitDelete LimitKind = 18
)

// Limit is a generic representation of a limit and its value.
type Limit struct {
	Kind  LimitKind
	Value *int64
}

// ProjectUsageLimits holds project usage limits and current usage.
type ProjectUsageLimits struct {
	StorageLimit          int64  `json:"storageLimit"`
	UserSetStorageLimit   *int64 `json:"userSetStorageLimit"`
	BandwidthLimit        int64  `json:"bandwidthLimit"`
	UserSetBandwidthLimit *int64 `json:"userSetBandwidthLimit"`
	StorageUsed           int64  `json:"storageUsed"`
	BandwidthUsed         int64  `json:"bandwidthUsed"`
	ObjectCount           int64  `json:"objectCount"`
	SegmentCount          int64  `json:"segmentCount"`
	RateLimit             int64  `json:"rateLimit"`
	SegmentLimit          int64  `json:"segmentLimit"`
	RateUsed              int64  `json:"rateUsed"`
	SegmentUsed           int64  `json:"segmentUsed"`
	BucketsUsed           int64  `json:"bucketsUsed"`
	BucketsLimit          int64  `json:"bucketsLimit"`
}

// UsageLimits represents storage, bandwidth, and segment limits imposed on an entity.
type UsageLimits struct {
	Storage               int64  `json:"storage"`
	UserSetStorageLimit   *int64 `json:"userSetStorageLimit"`
	Bandwidth             int64  `json:"bandwidth"`
	UserSetBandwidthLimit *int64 `json:"userSetBandwidthLimit"`
	Segment               int64  `json:"segment"`
	RateLimit             *int   `json:"rateLimit"`
	BurstLimit            *int   `json:"burstLimit"`
	RateLimitHead         *int   `json:"rateLimitHead"`
	RateLimitList         *int   `json:"rateLimitList"`
	RateLimitGet          *int   `json:"rateLimitGet"`
	RateLimitPut          *int   `json:"rateLimitPut"`
	RateLimitDelete       *int   `json:"rateLimitDelete"`
	BurstLimitHead        *int   `json:"burstLimitHead"`
	BurstLimitList        *int   `json:"burstLimitList"`
	BurstLimitGet         *int   `json:"burstLimitGet"`
	BurstLimitPut         *int   `json:"burstLimitPut"`
	BurstLimitDelete      *int   `json:"burstLimitDelete"`
}
