// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1642600843"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "228f465d4559563a9f0d96be6dde4ecc08fe120e"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.47.0-rc"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
