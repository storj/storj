// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1683033015"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "defb9eae824787bce6ceadcba50a8cd45b4e0a6b"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.78.1"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
