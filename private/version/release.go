// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1618437567"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "21c95bf1669886b44ef531b3f2fb5f210d0febb6"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.27.4"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
