// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1673435563"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "e9ca27f77fbe6195b1bd10eb1d898363d0a2a8f4"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.70.2"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
