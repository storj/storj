// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1658948369"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "ed78d4e6b14e695aba97056d0ab4e3867af39686"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.60.3"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
