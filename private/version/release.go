// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1736186038"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "4bde5b88d47c7d2dd0a4a57bd023b72ee4b46070"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.119.14"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
