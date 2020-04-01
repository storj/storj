// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1585753217"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "ae36a0c2a7272342dab269933dc86181de7a62c4"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.1.1"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
