// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1606699264"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "ba920a0cd022b8a7443df506179ee76275a99161"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.17.7-rc-3"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
