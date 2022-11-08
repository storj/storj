// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1667912830"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "986c3490f0c77f23415fa7306ace789b1e8ae447"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.66.5-rc"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
