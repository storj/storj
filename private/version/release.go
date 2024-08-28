// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1724834025"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "eda810d99c4220d2292e60f81945d74d5aca1113"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.112.0-rc"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
