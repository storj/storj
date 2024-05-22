// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1716402001"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "6c09cd15c453db85420387c8726dd8f6c1612df7"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.104.6-rc-2"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
