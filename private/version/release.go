// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1619534043"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "c17ab0fe3bc4804e079a5d4c4e9d5f5bbcff29f8"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.28.2"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
