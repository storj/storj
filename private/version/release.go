// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1690910649"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "bf0f3b829f699bc5fc7029c4acf747e7857e13d8"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.84.2"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
