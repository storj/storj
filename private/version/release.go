// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1612390922"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "4bc5d51fa6dae7c7c529381dea3af70d42a69646"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.21.4"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
