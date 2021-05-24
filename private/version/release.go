// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1621892667"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "ce977e1498b98d4f3fbd80cc3572083c06130872"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.30.3"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
