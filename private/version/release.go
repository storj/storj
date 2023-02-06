// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1675705541"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "b3953f618af02f6e4d58a64f542b7940e6b0de28"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.72.2"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
