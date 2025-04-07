// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1744029980"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "d42da241e2959a6c9ed992955140f1e7bc011400"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.126.2"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
