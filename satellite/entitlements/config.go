// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package entitlements

// Config holds the configuration for the entitlements service.
type Config struct {
	Enabled bool `help:"indicates whether the entitlements service is enabled" default:"false"`
}
