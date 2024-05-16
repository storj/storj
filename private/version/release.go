// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1715891867"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "7ee1de024f7690f59557e423f380232cd87b6e2d"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.104.6-rc"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
