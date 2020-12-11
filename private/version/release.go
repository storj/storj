// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1607694944"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "ca1e6b9756f4a410392a59bf86c878feb7f87ae4"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.19.0-rc"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
