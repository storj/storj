// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1620905839"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "d32ae0459b3c63e957de66b5adc09aaaf5a4870f"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.30.0-rc"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
