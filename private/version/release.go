// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1606933068"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "8468843ceb1cf605ccc61c2403323b38544d22ff"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.18.2-rc-1"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
