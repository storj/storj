// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1714484620"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "30ad064e0cb9fa9c109f0d5c2c3f40e523bb393e"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.103.2"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
