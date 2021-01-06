// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1609956042"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "c4528336881eff440e503d8bb592f9c9e9fa3447"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.19.8"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
