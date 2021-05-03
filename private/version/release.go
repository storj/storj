// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1620070106"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "7078d955873202f28b213c1e03b91d465218f0fa"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.29.2"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
