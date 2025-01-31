// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1738357448"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "e01667c41c47e559e2a04202a1183a983517b2c3"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.121.4"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
