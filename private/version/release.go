// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1740071225"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "52e10bdbe9b3bdd3f43087fc8dda8fa316acbba0"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.123.2-rc"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
