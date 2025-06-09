// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1749498183"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "1654d214633325b3dd2f04e291d25329d05ddb79"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.130.8"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
