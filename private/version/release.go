// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1684161058"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "87d0789691c2571daa5ead2344111f90a2281c41"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.79.1"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
