// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1739555904"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "393576ecc7e11e84cdf3d7dac72df42b8b0d3476"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.122.6"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
