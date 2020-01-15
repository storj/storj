// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package preflight

import (
	"gopkg.in/spacemonkeygo/monkit.v2"
)

var mon = monkit.Package()

// Config for graceful exit
type Config struct {
	EnabledLocalTime bool `help:"whether or not preflight check for local system clock is enabled on the satellite side. When disabling this feature, your storagenode may not setup correctly." default:"true"`
}
