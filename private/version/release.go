// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1587565816"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "0ab8c64a51724fe8be4edfc1529fc7248bc2c160"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.3.2"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
