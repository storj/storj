// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1732291351"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "3a8b8399a4dfb36eb85916af436ae0903cc06b69"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.118.2-rc"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
