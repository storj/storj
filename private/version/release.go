// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1619685197"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "30d2a2ea7b384bb616ea2e9dc1f897c8c4284b64"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.29.0-rc"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
