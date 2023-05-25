// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1685002362"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "6f2d6a97a606d331d6f25fa609b243e09b00579b"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.80.0-rc"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
