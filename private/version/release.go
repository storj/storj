// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1609790290"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "c131c64f8107213b7a0d9c2edf13dece8040fd39"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.19.4-rc-multipart"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
