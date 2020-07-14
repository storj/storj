// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1594746014"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "325925c0b4e8baa666558dc544d43f71da2d92ed"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.8.1"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
