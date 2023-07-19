// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1689791523"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "4cb85186b2b6fd353d9449238a4690a1560125b0"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.84.0-rc"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
