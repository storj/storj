// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1685083408"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "3cd5e13125ff8b0e753b22dfccd331715fbc0c5d"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.80.2-rc"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
