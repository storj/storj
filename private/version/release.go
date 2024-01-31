// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1706705248"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "143abf13936f6521f9edd070c5f56d2589d1b157"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.96.6"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
