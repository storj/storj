// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1713808267"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "f84c2cf86a34c5afe2bd4b7379f5bd45e3e817c8"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.102.4"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
