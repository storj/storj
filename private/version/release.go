// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1595587734"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "65df67701ca4ece0b36237f0d11d1d5a847921ec"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.9.1"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
