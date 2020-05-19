// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1589894030"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "c916d4e69ffb70fc1c242b488d06652b49236984"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.4.5"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
