// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1752677036"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "ce558463094050982fd4a13d9565045304d705f4"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.133.5"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
