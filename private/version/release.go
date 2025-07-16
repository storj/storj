// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1752668584"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "b76d75374ca6c39cd82f578c3e2baf63b32c607f"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.133.4"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
