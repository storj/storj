// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1755016923"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "73c7102a70d3eb4b05d2e85d422e0f8b9ce47cb2"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.135.5"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
