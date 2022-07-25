// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1658749982"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "b5561b1c3448a8d7134e9fa6973230eb0a6acb1d"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.60.1"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
