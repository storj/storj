// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1732301481"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "4b1e6261ac392fd21a51de092fbe802740b3e0dc"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.118.3-rc-test"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
