// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1738882480"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "58b2f4b7c70d446cc5afe6cec7afd1d5e9a2ba9f"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.121.8"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
