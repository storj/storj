// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1734455845"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "868864fd3440c7fe3001aa0f4810d5c666d40221"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.119.6"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
