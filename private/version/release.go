// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1763379288"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "0b56d4c03ac80ce846c5dfb4fc9c4d1ae8ced9be"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.142.3"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
