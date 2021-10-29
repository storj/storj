// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1635513053"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "8210f112622433dfc370039bcaf7e6ca6bee1da9"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.42.1-rc"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
