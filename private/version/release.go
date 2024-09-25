// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1727290354"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "d0cca36826b1625a00dbe88399c438dd7d898ee8"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.113.5"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
