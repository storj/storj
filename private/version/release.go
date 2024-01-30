// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1706602068"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "1b8949c51de72d3f53932300115da68609be29ac"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.96.4"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
