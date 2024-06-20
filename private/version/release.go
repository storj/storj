// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1718871870"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "b92bbb1f5926261f9d419f07c7af1ebc5b6b707f"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.107.0-rc"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
