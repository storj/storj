// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1705590073"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "8dd73e1c8f16741afb5017cc79d9e116ec119cfc"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.96.1-rc"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
