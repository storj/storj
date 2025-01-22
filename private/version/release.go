// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1737550841"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "2b91c5d2a7f4466f9ade86f49db1e0f1cfc8d114"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.120.7"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
