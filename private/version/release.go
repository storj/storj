// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1645024916"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "b181db6c2b303862c2556a260f09d42a6ccb63af"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.49.0-rc"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
