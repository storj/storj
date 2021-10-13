// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1634140239"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "b8dd35ceaf8d2c71c8fefd6641d8ef3ad4f30075"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.41.0-rc"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
