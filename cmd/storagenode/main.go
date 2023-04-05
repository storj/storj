// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"go.uber.org/zap"

	"storj.io/private/process"
	_ "storj.io/storj/private/version" // This attaches version information during release builds.
)

func main() {
	process.SetHardcodedApplicationName("storagenode")

	if startAsService() {
		return
	}

	rootCmd, _ := newRootCmd(true)

	loggerFunc := func(logger *zap.Logger) *zap.Logger {
		return logger.With(zap.String("Process", rootCmd.Use))
	}

	process.ExecWithCustomConfigAndLogger(rootCmd, false, process.LoadConfig, loggerFunc)
}
