// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1626200887"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "b8696603ce377e8292a1448a46ad93bde7deae6f"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.34.3"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
