// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package eventingconfig

import (
	"time"
)

// Config contains configuration for bucket eventing.
type Config struct {
	Cache CacheConfig `help:"cache configuration for bucket notification configs"`
}

// CacheConfig contains configuration for the bucket notification config cache.
type CacheConfig struct {
	// TTL is how long a configuration entry is cached. Configuration changes may
	// take up to this duration to propagate across all pods.
	TTL time.Duration `help:"TTL for cached bucket notification configs" default:"1m"`
	// Capacity is the maximum number of entries in the cache.
	Capacity int `help:"maximum number of entries in the in-memory config cache" default:"10000"`
}
