// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1760950445"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "f976e45b4c7f9bf96e327620bd89b5d04744dd23"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.140.2-rc"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
