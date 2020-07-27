// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1595882626"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "b90bf0eeaf5a128b89f12057c337c6778d81ab1a"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.9.2"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
