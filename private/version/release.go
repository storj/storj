// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1680512318"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "fd5b30fe128794580609c0de4977cd6f5d87b066"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.76.2"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
