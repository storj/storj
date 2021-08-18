// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1629305656"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "ef9a5210a4e929824e2f63b0029e0473678e5b79"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.37.0-rc"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
