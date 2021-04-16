// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1618576754"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "8ffb34b8f93ca76f863f0c264f4241c1d6143a1f"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.27.5"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
