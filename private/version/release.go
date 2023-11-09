// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1699513884"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "404bddd2a4bc3d010e636ca0eeb3487a9caec2d9"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.92.0-rc"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
