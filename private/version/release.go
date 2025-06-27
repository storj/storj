// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1751031530"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "00dd89069e213fcbb792498a28128dd76cb1784f"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.132.2"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
