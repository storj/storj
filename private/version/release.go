// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1739923865"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "defa2d979d1f7b61d37fa8b9a4a736dd0318c714"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.122.9"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
