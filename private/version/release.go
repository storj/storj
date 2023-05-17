// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1684345922"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "2f99b7e74844aa9acea80319839b6118bedeb760"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.79.3"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
