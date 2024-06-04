// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1717511352"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "db39d5d2fc0ac985e69542831868d787582e1fa1"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.105.9-rc"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
