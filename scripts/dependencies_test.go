// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package scripts_test

// this ensures that we download the necessary packages for the tools in scripts folder
// without actually being a binary

import (
	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/imports"
)

var _ = imports.Process
var _ = packages.LoadImports
var _ = astutil.PathEnclosingInterval
