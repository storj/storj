// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1613485736"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "a7966433c3c57a1dc60177540b5de2d6cba2ead9"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.23.1-rc-multipart"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
