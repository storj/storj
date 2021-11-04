// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1636069893"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "1a89c90fcc0c47e9fd81a76bec4f5cac401e3479"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.42.4"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
