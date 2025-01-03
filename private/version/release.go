// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1735917715"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "83c2e8fb464967b33a91a3caa9bac8d4601efda1"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.119.12"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
