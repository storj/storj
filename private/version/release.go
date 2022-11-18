// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1668787413"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "301977a6f29b1f8402ec840dca2f8592965e7442"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.67.3"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
