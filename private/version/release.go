// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1715274853"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "44c8bd78002e8cc5a33d0218da7ab3cba1d24511"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.104.1"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
