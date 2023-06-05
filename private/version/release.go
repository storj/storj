// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1685993112"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "f0c5aaa69a30325a85f854ff7a39476a667d8048"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.80.7"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
