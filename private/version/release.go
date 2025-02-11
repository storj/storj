// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1739309072"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "c13900b66b22e675f20c0f6e8c64504e3661334a"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.122.4"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
