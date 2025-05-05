// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1746460035"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "7799fea55b0bbd9af739e1a5a90e86748d1797c4"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.128.2-rc"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
