// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"os"

	"go.uber.org/zap"

	"storj.io/private/process"
	_ "storj.io/storj/private/version" // This attaches version information during release builds.
)

func main() {
	process.SetHardcodedApplicationName("storagenode")

	if startAsService() {
		return
	}

	allowDefaults := !isFilewalkerCommand()
	rootCmd, _ := newRootCmd(allowDefaults)

	loggerFunc := func(logger *zap.Logger) *zap.Logger {
		return logger.With(zap.String("Process", rootCmd.Use))
	}

	process.ExecWithCustomOptions(rootCmd, process.ExecOptions{
		InitDefaultDebugServer: allowDefaults,
		InitTracing:            allowDefaults,
		InitProfiler:           allowDefaults,
		LoggerFactory:          loggerFunc,
		LoadConfig:             process.LoadConfig,
	})
}

func isFilewalkerCommand() bool {
	return len(os.Args) > 1 && os.Args[1] == usedSpaceFilewalkerCmd
}
