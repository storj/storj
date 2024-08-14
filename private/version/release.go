// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1723620262"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "be0c7a264c302e992e4e03bca6b2107805352a97"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.111.0-rc"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
