// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1633511162"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "744a2e7e9402af87ad308b0a99ad6d31570b6c62"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.40.4"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
