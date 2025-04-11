// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1744355526"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "7ad8a6c43cfb0a429444c9e6281cef2cd6f0b531"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.126.4"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
