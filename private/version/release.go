// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1656516347"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "6de7824c1047ba924f34154e54cd68dca13d1e78"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.58.1"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
