// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1618246179"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "4e8f9623a25d54859928be1e60845787be5f9c6b"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.27.1"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
