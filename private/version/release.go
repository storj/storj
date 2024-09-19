// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1726762389"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "03dd428e33f5b185d2f786f5b6ecdba04756b68a"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.113.3"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
