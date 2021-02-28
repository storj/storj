// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1614508532"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "b557308b87725a904f444abbd4334b5fa0b41157"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.24.6-rc-multipart"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
