// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1748619009"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "4119aa2a2e7bcf5ce1745817b9a77bcef09ccba4"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.129.5"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
