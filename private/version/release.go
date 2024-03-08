// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1709910083"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "7572c529eaf34b488fa5c82c07f5930ff66daf6a"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.99.3"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
