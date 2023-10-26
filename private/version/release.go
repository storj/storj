// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1698304989"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "f5d717735b502245f7ef3f05626defe21ac6fb19"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.91.0-rc"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
