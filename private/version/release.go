// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1649854635"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "8e0196a6cc0895457bf94aea9a9bb6cedabe3421"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.53.0-rc"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
