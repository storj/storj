// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1756885550"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "47f862b4eef5dd23fed242539f4225f32dc6e305"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.137.0-rc"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
