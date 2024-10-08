// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1728428474"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "1a2e658f53ec57fd2e4c461ebf5537b206ba22e3"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.114.6"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
