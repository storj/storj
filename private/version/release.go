// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1760010825"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "e10d7725ac66daf7be8c9d1ba64172dda7af8b94"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.139.6"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
