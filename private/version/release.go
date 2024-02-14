// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1707906653"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "75786256d01c9310b0e975d07034963d90e0b192"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.98.0-rc"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
