// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1725351688"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "c5e70201da8f3c40f1b72fb9e68c63e7c5fa8fe5"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.112.1-rc"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
