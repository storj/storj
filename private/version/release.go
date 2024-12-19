// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1734581316"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "86bc293f29c47021914607d6dc9b361d29acb7b1"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.119.8"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
