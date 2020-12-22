// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1608652046"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "9a8959d4298bb18b5b304a40b3184e6735a5c8b2"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.19.3-rc-multipart"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
