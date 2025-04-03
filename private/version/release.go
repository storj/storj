// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1743674569"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "1cd21be31400343b714ff2f3d8f3e935552e1d8f"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.125.4"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
