// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1627653197"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "d95eab288e229e42f877ab53de37d5a3b4b53198"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.35.3"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
