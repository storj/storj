// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1752049910"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "177e656f48fe3bd41d2d1bf22e8d66692ce0433f"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.133.0-rc"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
