// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1591217422"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "90f9c26df26b72ffce77a15474f9bb10be5c91d5"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.6.2"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
