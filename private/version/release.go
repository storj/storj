// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1747146758"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "5a87fe1ce1d98953d41f9bb32340d56161c74cb5"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.128.6"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
