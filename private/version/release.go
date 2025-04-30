// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1746028308"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "e7d7c087c6cd8b8e8cb94c1fa53934abea6fdaca"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.128.0-rc"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
