// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1587998403"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "c5bd8a1912077c8ac82d0da1cc0684276629127f"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.3.4"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
