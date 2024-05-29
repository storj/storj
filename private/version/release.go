// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1716982317"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "1f9e0c9c0a4c198edcdd6f179466cd9fb47fcaf3"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.105.4"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
