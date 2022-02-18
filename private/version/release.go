// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1645184169"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "f84552c2ede42aa3759ea6c1d2e688944b31eb5b"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.49.2-rc"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
