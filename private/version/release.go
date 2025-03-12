// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1741791474"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "e33422d803fc497e10fd3c02d26bf4e4bc0d1243"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.124.5"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
