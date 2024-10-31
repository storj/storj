// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1730369441"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "0a0eb3aea1a313dfc5d7fc7b03a7acd1c4be59b8"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.116.4"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
