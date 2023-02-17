// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1676657095"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "fcdc8b9c66f53caab77e36af0e6f901f7182500b"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.73.1-rc"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
