// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1653565219"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "4725a3878c42e504cdf0071bff8a76043dfc07f5"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.56.0-rc"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
