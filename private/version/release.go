// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1734619466"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "1261ba1b8974d36340f8ff3cb796aa92506660e4"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.119.9"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
