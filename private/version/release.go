// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1716481640"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "df582371dd80977f1a405edcbec851be6eae88c5"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.104.7-rc"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
