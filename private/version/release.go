// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1593556238"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "3567b49ef4dc92f4316a8a6469697a3ac20da04e"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.7.0-rc"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
