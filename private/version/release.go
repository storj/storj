// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1760100464"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "b0d6b3a04b1655cd7f69ce14a4e736a4ae97743e"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.139.7"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
