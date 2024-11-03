// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1730643045"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "a545e7f160f20a006686a408dd8b1a4fb9f8ed9d"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.115.6"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
