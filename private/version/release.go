// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1602087032"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "5ab205bd64e18ee0f5f684f54b7029376563b40c"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.14.2"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
