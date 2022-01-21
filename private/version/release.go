// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1642770083"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "51c787d62ba51c1309177c6e73f9b9187d3c5c9c"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.47.2-rc"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
