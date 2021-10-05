// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1633420973"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "3032e1b75755a3d8f2cf1b56e7435ec185d54844"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.40.3"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
