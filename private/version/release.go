// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1603114087"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "6a4a288c067b277eb21146bfc626079778f36a65"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.14.8"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
