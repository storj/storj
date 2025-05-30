// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1748597893"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "d8fc4b72effb77aca9c0fa0289b30daae2a6f6f8"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.130.1"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
