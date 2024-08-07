// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1723199056"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "de2da0ab1f1a0c56f0671741b3d0f68df5c84508"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.110.3"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
