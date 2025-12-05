// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package healthcheck

// Config is the configuration for healthcheck service and endpoint.
type Config struct {
	Details bool `user:"true" help:"Enable additional details about the satellite connections via the HTTP healthcheck." default:"false"`
	Enabled bool `user:"true" help:"Provide health endpoint (including suspension/audit failures) on main public port, but HTTP protocol." default:"true"`
}
