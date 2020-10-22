// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1603372762"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "a4720132f5f57498f3bda40e9bd6796a52466b21"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.15.2"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
