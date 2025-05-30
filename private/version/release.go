// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1748634719"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "649750090c97b1560f0f9aace65ccf641bdc93da"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.130.3"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
