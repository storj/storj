// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1614299161"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "f013bf9a3679bac59348655b9a1f9a6401398b0b"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.24.5-rc-multipart"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
