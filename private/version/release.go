// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1661266153"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "fd9c037476738e4013c42db6ff3575de8a382cdf"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.62.3"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
