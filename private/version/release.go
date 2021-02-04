// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1612451350"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "615586a4712dea11c0df076126c15ff356236b38"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.22.0-rc"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
