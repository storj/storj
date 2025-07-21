// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1753109042"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "4e18aed6f30408da1c7e3524f459bbe1a76d761c"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.132.9"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
