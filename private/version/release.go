// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1727255044"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "29f55969cf6512d108ad00a2f71ca799334accd6"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.114.0-rc"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
