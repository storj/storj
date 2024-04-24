// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build !windows

package main

import (
	"go.uber.org/zap"

	"storj.io/common/process"
)

func main() {
	logger, _, _ := process.NewLogger("storagenode-updater")
	zap.ReplaceGlobals(logger)

	loggerFunc := func(logger *zap.Logger) *zap.Logger {
		return logger.With(zap.String("Process", updaterServiceName))
	}

	process.ExecWithCustomConfigAndLogger(rootCmd, true, process.LoadConfig, loggerFunc)
}
