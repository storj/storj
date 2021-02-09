// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1612883131"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "0ddd937b6e5595de57ccb6cd3551965820a87749"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.22.1-rc-multipart"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
