// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1684270004"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "b4f0e9dbb839ec5412cf073fc172686e68fd1b9b"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.79.2"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
