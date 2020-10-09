// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1602278190"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "8f2adc2a172a2a5f8d6c7b5acc1b903504d695f8"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.14.6"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
