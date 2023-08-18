// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1692357737"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "e5854062a4c8b793b72ae1bbc083746a7ae16e3a"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.86.1"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
