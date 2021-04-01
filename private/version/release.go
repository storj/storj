// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1617301070"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "8e470d6c8980583c7c2bcb6fa3201ae3cd8cccda"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.26.1-rc"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
