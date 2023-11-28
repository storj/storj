// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1701163661"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "2e5bcfef6a79a37170efd215db87e12f36388c29"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.93.1"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
