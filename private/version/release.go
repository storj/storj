// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1763976710"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "78b13a23f468da6a91c32a42cbf3d93c061a4167"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.142.6"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
