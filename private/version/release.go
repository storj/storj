// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
