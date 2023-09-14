// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1694708175"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "4134641c510ed5973cf3acd81aa14360be1f310d"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.88.1-rc"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
