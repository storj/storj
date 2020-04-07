// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1586267768"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "80ee7321cd8d2f71f67b03bd00399a25fb281b7a"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.2.0-rc"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
