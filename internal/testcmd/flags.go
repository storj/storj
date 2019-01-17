package testcmd

import "flag"

var (
	Integration      = flag.Bool("integration", false, "run tests that exercise built cli binaries")
	DefaultDirs      = flag.Bool("default-dirs", false, "run tests which execure commands that read/write files in the default directories")
	PrebuiltTestCmds = flag.Bool("prebuilt-test-cmds", false, "run tests using pre-built cli command binaries")
)

func Noop() {}
