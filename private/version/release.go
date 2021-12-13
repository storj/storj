// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1639410370"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "05e86e559bd356d529e40a00d5505f058c1e630f"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.45.3"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
