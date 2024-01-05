// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1704468952"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "f213b3827adcf9f1ee3e09dbef1cb59122e1783a"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.95.1"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
