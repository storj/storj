// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1751039872"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "2fe08d26fa7be7f499254aa226d655f5611cfdbc"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.132.3-rc"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
