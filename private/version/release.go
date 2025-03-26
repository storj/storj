// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1743015709"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "11b5a2d9482d72383b572fd87990e8b6a4f3a63d"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.126.0-rc"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
