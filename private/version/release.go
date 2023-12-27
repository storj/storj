// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1703693252"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "ded0b93eb4000f34b58ca548d6a76d32f3b31d79"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.94.2"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
