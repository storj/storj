// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1729610024"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "d4cfe2932e403f1e3a56d6df6d6115be08b5c553"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.115.2"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
