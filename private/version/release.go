// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1710494785"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "53b57d93b9a9977a58eec9bd6700fd0ad59e298e"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.100.2"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
