// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1732019082"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "929d0bf5ef272229917e70bce177168b8bfbd362"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.117.7"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
