// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1628088085"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "f5ac00f909ff4e82f2f024b2ad32966dcbd70a49"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.36.0-rc"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
