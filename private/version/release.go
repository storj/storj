// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1643648231"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "08e61e517000c385b72d0956075d88f38a63c09a"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.47.4"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
