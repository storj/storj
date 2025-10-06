// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1759740583"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "915fc3e3ec87dff6423c99618996b4e0cce13c97"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.139.3"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
