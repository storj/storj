// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1622470933"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "3af91e7a90c113d191bac990fbfc0b1a393a50f3"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.31.1"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
