// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1747744453"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "b4faf7f2884049e24bb0dd15af95f010642c6195"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.129.3"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
