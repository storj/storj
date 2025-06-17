// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1750144531"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "f337c9da0dd50f47e683696a5cb668b4936a8dc7"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.131.4"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
