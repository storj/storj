// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1750063833"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "2fa79f75a9330f3ac776cd258609f26bec7f204a"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.131.3"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
