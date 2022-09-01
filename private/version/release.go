// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1662052212"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "78c496ba8c1a0af6f827f0649e364b9d4bf05823"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.63.1"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
