// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1724324186"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "a45f3c902a7ec2305df50e264987b2d250a40df6"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.111.5"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
