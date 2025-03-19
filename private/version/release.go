// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1742379327"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "d52877ee93a147c37033066b3c81000d775e2e64"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.125.0-rc"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
