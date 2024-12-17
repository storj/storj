// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1734426920"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "591387b3a94ca71121e7ad8bf122dc3f2549ca3e"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.119.4"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
