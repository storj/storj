// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1747573695"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "9358cded765a177a4b3199ec82e62d1c5a81cc77"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.128.7"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
