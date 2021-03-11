// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1615478989"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "24c852771a9ed1b611629bd274594c6f0ec692eb"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.25.2"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
