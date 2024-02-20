// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1708453514"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "1c1b68c0c21b4275073ebe471c59b967d77458eb"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.98.2"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
