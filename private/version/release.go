// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1686000622"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "97f7fb5f887193362a0619ab64e09cce035f4173"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.80.9"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
