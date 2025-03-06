// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1741277994"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "afb4abd8c73b5386f3f96f91a4a2872e3b510fb2"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.124.2-rc"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
