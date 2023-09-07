// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1694127567"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "a60ea96c504705b963f2e8ba45db8bce17a6d05b"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.87.3"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
