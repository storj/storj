// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1723663779"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "d1a5c71671e3698178c212f6e44ab1e50974a697"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.110.5"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
