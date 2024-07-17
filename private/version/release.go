// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1721196614"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "3276989dbe417f11ff1f6a8c3482e06b0e617bb7"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.109.0-rc"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
