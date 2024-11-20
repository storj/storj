// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1732096195"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "5069cd39960367a3bd8c579f5a03b01f350c9544"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.118.0-rc"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
