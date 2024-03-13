// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1710325936"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "74adf59dc7443c6a6b734598a93c0ee6fa5c147d"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.100.0-rc"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
