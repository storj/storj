// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1720428566"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "252bc5acb54546ce8b0bba1d04d245be30b2dd48"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.108.1"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
