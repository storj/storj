// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1667483184"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "10ff37085d975b1832d5e00a7436d43e0ceb9680"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.66.3-rc"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
