// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1748423204"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "400d3caf1922cd80b5a35e1d537048421f8805a3"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.130.0-rc"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
