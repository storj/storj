// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1745996018"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "e210f6d7173b05fcaf55dd0d8376ab46d1335153"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.127.3"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
