// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1737541100"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "0ae5a0f42c54f114d80c137639dc30fc3aef25a5"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.121.0-rc"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
