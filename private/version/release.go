// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1609793589"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "ee0f66ef88a46a3237b1279eb24ad578cc70353e"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.19.5"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
