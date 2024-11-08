// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1731056018"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "4e34b7ade8f57ee91b02ba274beec0d1a0ac9f61"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.117.1-rc"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
