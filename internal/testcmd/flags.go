// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testcmd

import "flag"

var (
	// Integration is used to by tests to determine if "integration" tests should be run.
	Integration = flag.Bool("integration", false, "run tests that exercise built cli binaries")
	// DefaultDirs is used by tests to determine whether to run tests which modify files/dirs in default locations.
	DefaultDirs = flag.Bool("default-dirs", false, "run tests which execure commands that read/write files in the default directories")
	// PrebuiltTestCmds is used by tests to determine whether to build test cli binaries or expect them to alredy be installed.
	PrebuiltTestCmds = flag.Bool("prebuilt-test-cmds", false, "run tests using pre-built cli command binaries")
)

// Noop is an exported function that is used to allow these flags to be defined in tests
// which cause `flag.Parse` to be called but doesn't otherwise import this package. If these
// flags are not defined and are used against such tests, an error occurs.
func Noop() {}
