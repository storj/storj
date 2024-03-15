// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1710511039"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "ad361b7a3e849a8beb4f753053d70d90fd0dbce6"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.97.3"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
