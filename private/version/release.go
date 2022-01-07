// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1641590681"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "e8b6a5d02505b6249ebd21d0d0d5dcd19ab10587"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.46.2-rc"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
