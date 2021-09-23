// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1632436414"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "e16c2c9477415c1ceea644f251cd079d17d7e3c2"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.39.5"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
