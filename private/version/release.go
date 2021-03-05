// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1614952765"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "376547c33c7dff04b3a56cbd43f4b6035f4a661f"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.25.0-rc"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
