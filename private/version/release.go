// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1706718145"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "6e7e05772a4be9bfcc1ec179f0ab7d8aaeed8e3e"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.97.0-rc"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
