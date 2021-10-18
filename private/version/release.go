// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1634563309"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "9a219c3db7cf42227b88fe96e7808e9e62ce5e8e"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.41.2"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
