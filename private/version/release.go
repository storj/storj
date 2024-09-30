// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1727730261"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "ba239814de76a84d0e94081b73dfa2b139be1c18"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.114.3"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
