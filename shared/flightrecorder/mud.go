// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package flightrecorder

// NewTestConfig creates a new test config.
func NewTestConfig() Config {
	return Config{
		Enabled:              true,
		DBStackFrameCapacity: 1000,
	}
}
