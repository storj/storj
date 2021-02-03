// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1612380807"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "8527c0c7a9f5a5e635a73f0adeb833baec2424f1"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.21.3"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
