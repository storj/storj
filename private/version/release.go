// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1683102674"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "2690efdcc2943d5f73c20c04aeb0ab62dc5732b5"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.78.2"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
