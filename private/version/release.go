// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1608648919"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "5126b1a89f895ad2c83ef7300e132f9635787c47"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.19.4"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
