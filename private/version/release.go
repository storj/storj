// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1616515559"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "d298a4ba4b781103a097742a45ee234fd8c0a509"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.25.4-rc-multipart"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
