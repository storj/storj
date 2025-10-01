// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1759302356"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "8401b098a67d94b9567f03801b85759c1fb9c19f"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.138.3"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
