// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1762951404"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "ad35e7e5c589f5a793c9a2cd1232f770c3de61c2"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.142.1-rc"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
