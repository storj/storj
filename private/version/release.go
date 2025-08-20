// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1755677947"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "f5ca23400deb62fc2cbc26507a710f530e126025"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.136.0-rc"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
