// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1678111545"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "b46c0fb78f83f53a9cb7ce1ce9092b6f3facc467"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.74.1"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
