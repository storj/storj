// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1614513640"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "bfa8bab7d2d6608e7389724ccad04f70cc79b6ad"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.24.7-rc-multipart"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
