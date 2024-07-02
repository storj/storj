// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1719940726"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "d18201b5a252b7773efbf07b8f7c08cb70e31494"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.108.0-rc"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
