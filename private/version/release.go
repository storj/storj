// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "1739265090"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "4f31fcc121f0ab19c278124e0b5dba83e41f2ba6"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "v1.122.2"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
