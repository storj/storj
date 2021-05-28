// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package preflight

import (
	"github.com/spacemonkeygo/monkit/v3"
)

var mon = monkit.Package()

// Config for preflight checks.
type Config struct {
	LocalTimeCheck bool `help:"whether or not preflight check for local system clock is enabled on the satellite side. When disabling this feature, your storagenode may not setup correctly." default:"true"`
	DatabaseCheck  bool `help:"whether or not preflight check for database is enabled." default:"true"`
}
