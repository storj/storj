// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1723810460"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "3cc9cd0b4a123e2911150e2bd609e5603e8c19b0"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.111.3-rc"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
