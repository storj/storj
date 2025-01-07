// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1736262533"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "2da499730ee4ff6f01c8ae02be1e13abdb30e6c5"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.119.15"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
