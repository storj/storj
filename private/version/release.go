// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1599825564"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "202c2d63c46bb3c21cc048fa5d4895b050c666df"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.12.6"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
