// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1589904656"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "671aca56b0b4977776c7adbeb515c022aa9b4e3b"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.5.0-rc"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
