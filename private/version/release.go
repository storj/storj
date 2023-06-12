// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1686577230"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "3959796f6fe10b7aea320847f9b2b9c54afc9ca3"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.81.2"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
