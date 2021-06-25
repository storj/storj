// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1624580241"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "e37df4491074e791fbb9a38342d1f3f19739cfcb"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.33.0-rc"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
