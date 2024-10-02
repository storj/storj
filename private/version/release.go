// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1727858177"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "b72002acfe77f9747c930a77d7b2a3c6e0c7b17d"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.114.5"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
