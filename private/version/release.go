// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1731431942"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "e492807ed7c6c54d5489b92325ff49600a0a95f1"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.117.5"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
