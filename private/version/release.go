// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1675877688"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "fb677e784b858be386d65c4c6e6f71a8fedbcac8"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.72.4"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
