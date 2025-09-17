// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1758094778"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "e84a0899902c7403d6b6a38b70489d68f63bc459"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.138.0-rc"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
