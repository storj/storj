// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1606236248"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "6a64c24f55995023ab570c54f45342849f7fa281"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.17.5-rc"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
