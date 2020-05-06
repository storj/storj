// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1588788847"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "32d9bf0f361c9f68f7ab1f6374c50d1e190adf86"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.4.2"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
