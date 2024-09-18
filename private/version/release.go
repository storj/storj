// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1726647887"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "91cdd187a31431e058e77eb34c4e7dd8683e3141"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.113.2"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
