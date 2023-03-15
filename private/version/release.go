// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1678906932"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "132a688becc040043d2460e5d095ce58b8eeef10"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.75.0-rc"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
