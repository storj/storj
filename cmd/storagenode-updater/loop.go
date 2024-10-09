// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build unittest || !windows

package main

import (
	"context"

	"go.uber.org/zap"

	"storj.io/storj/private/version/checker"
)

// loopFunc is func that is run by the update cycle.
func loopFunc(ctx context.Context) error {
	zap.L().Info("Downloading versions.", zap.String("Server Address", runCfg.Version.ServerAddress))

	all, err := checker.New(runCfg.Version.ClientConfig).All(ctx)
	if err != nil {
		zap.L().Error("Error retrieving version info.", zap.Error(err))
		return nil
	}

	if err := update(ctx, runCfg.RestartMethod, runCfg.ServiceName, runCfg.BinaryLocation, runCfg.BinaryStoreDir, all.Processes.Storagenode); err != nil {
		// don't finish loop in case of error just wait for another execution
		zap.L().Error("Error updating service.", zap.String("Service", runCfg.ServiceName), zap.Error(err))
	}

	if err := update(ctx, runCfg.RestartMethod, updaterServiceName, updaterBinaryPath, runCfg.BinaryStoreDir, all.Processes.StoragenodeUpdater); err != nil {
		// don't finish loop in case of error just wait for another execution
		zap.L().Error("Error updating service.", zap.String("Service", updaterServiceName), zap.Error(err))
	}

	return nil
}
