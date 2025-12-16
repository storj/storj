// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package eventingconfig

import "time"

// Config contains configuration for bucket eventing.
type Config struct {
	Buckets  BucketLocationTopicIDMap `help:"defines which buckets are monitored for events (comma separated list of \"project_id:bucket_name:topic_id\")" default:""`
	Projects ProjectSet               `help:"defines which projects are enabled for bucket eventing (comma separated list of project UUIDs)" default:""`
	Cache    CacheConfig              `help:"cache configuration for bucket notification configs"`
}

// CacheConfig contains configuration for the bucket notification config cache.
type CacheConfig struct {
	Address string        `help:"Redis address for bucket notification config cache" default:""`
	TTL     time.Duration `help:"TTL for cached bucket notification configs" default:"5m"`
}
