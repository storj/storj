// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1753693858"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "7ba463300612c2e6b2c23b6ab583b2490d45896d"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.134.2"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
