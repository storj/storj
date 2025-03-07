// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1741378189"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "fa3b8c56a9ac9da4c727ac568e765c53521c5fdf"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.124.3-rc"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
