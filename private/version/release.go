// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1712251011"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "a452291e1b9f9b4e8399804c886f421b68927bd5"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.101.3"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
