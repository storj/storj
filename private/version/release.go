// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1713971193"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "86f85a9606a8f4a82bd3f9c3d994296cfc31345b"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.103.1-rc"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
