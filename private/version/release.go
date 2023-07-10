// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1689005431"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "61968718a5fe4c31b72a8a2cf85b7059e1af52b9"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.83.2"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
