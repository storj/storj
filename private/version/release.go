// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1723663873"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "a6ecb259550f0a9020f9976b8004e4de13135d2f"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.111.2-rc"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
