// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1608554863"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "65ba57b097d2d34860a3c94919b1ece12bb23ce4"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.19.3"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
