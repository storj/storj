// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1719401180"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "c59f6f879cac35d60c7565841f62ac077619aaf1"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.107.2"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
