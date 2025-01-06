// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1736183042"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "4233043c40ceae87d350ff2d22e8c0609a069c67"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.119.13"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
