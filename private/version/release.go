// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1738162325"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "abeaa921071132ca53efc7f9769dd889697b9543"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.121.3"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
