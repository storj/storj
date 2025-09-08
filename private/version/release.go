// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1757328211"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "f2f6eb9582f9de8ffea3c4f24a6318020869ea52"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.137.2"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
