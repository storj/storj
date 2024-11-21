// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1732218173"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "ff7b8bca438067422093c571f57139b67d8c5900"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.118.1-rc"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
