// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1756976119"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "699b11a5c320464ddab3bf917a2b73496bfb8c5e"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.136.5"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
