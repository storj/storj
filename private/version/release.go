// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1691421440"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "fac522d8dd2686bcfd72f6d18e2a41da07ea3f93"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.85.1"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
