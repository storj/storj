// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1685988076"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "cc444c2c523ef466d70f894f5483758f2b5f9cd7"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.80.6"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
