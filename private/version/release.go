// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1709111057"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "9380fe8f2f83d6e00e682a91deae5b3a03094a09"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.99.0-rc"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
