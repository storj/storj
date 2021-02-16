// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1613497474"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "55e639fda38d198eca0c641a956da776b05b79f8"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.23.2-rc-multipart"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
