// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1605289582"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "c8a5add246e44131452552cec734b70b0409e6f6"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.17.1"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
