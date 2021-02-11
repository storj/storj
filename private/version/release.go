// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1613087634"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "fe5b6e1725540bd651039fd4bf170dc73a2d936b"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.23.0-rc"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
