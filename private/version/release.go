// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1626689966"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "13db0251042081e9e5b37b272c463155a97ab417"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.34.5"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
