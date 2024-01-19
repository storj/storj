// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1705658167"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "dd52ef24478b9fee4ee882385ed46ad8fc1d7f99"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.96.2"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
