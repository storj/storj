// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1685999116"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "91964d9f1df7d8915d50e5853e06ad48e39b6195"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.80.8"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
