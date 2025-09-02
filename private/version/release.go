// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1756809614"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "c1b8a780eadc5663425320e07afac95b0222f4d8"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.136.4"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
