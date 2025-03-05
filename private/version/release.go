// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1741168563"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "a6f406d62d1ba2b4cefc6fea4b477e092f861614"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.123.6"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
