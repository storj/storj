// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1655448391"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "685147ce0e66c7121119f51a4e2bd9a347e3ed5e"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.57.2"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
