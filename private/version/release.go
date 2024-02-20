// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1708434582"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "026f56cb38516d7e75ad1a2995434cd7cf241ce4"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.98.1"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
