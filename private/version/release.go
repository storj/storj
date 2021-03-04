// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1614874025"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "3d20a6bb8c99decc27362ece078259db95a4f065"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.24.11-rc-multipart"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
