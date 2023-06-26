// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1687783565"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "e1f8434a03290f36202d40a3f887da1e4dc68ee5"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.82.1"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
