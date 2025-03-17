// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1742229121"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "6c652d6c7e4ea4e4b0d2f81c8fb16938f8d460f3"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.124.6"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
