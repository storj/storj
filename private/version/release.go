// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1589909299"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "941f2ac49c689dbe437a66901f68111626d08519"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.5.1-rc"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
