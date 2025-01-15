// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1736954385"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "bda4edae5f385df3f4c5d0f139453affa1599d8d"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.120.6-rc-test"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
