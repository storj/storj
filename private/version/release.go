// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/private/version.buildTimestamp
var buildTimestamp string = "1618849985"

//go:linkname buildCommitHash storj.io/private/version.buildCommitHash
var buildCommitHash string = "8bb0e2cd18b2b7bee3a57440db96a08bc0483cde"

//go:linkname buildVersion storj.io/private/version.buildVersion
var buildVersion string = "v1.27.6"

//go:linkname buildRelease storj.io/private/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
