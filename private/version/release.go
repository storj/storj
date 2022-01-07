// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1641556760"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "b8e8af7bb487cc2d4a792170663b61cf732e283f"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.46.1-rc"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
