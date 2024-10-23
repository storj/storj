// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1729664618"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "c77234853d0a54ccb009ced52d82130c03031d33"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.116.0-rc"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
