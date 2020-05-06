// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

// +build windows,!unittest

package main

import (
	"context"
	"os"
	"os/exec"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/private/version"
	"storj.io/storj/private/version/checker"
)

// loopFunc is func that is run by the update cycle.
func loopFunc(ctx context.Context) error {
	zap.L().Info("Downloading versions.", zap.String("Server Address", runCfg.ServerAddress))

	all, err := checker.New(runCfg.ClientConfig).All(ctx)
	if err != nil {
		zap.L().Error("Error retrieving version info.", zap.Error(err))
		return nil
	}

	if err := update(ctx, runCfg.ServiceName, runCfg.BinaryLocation, all.Processes.Storagenode); err != nil {
		// don't finish loop in case of error just wait for another execution
		zap.L().Error("Error updating service.", zap.String("Service", runCfg.ServiceName), zap.Error(err))
	}

	updaterBinName := os.Args[0]
	if err := updateSelf(ctx, updaterBinName, all.Processes.StoragenodeUpdater); err != nil {
		// don't finish loop in case of error just wait for another execution
		zap.L().Error("Error updating service.", zap.String("Service", updaterServiceName), zap.Error(err))
	}

	return nil
}

func updateSelf(ctx context.Context, binaryLocation string, ver version.Process) error {
	suggestedVersion, err := ver.Suggested.SemVer()
	if err != nil {
		return errs.Wrap(err)
	}

	currentVersion := version.Build.Version

	// should update
	if currentVersion.Compare(suggestedVersion) >= 0 {
		zap.L().Info("Version is up to date.", zap.String("Service", updaterServiceName))
		return nil
	}
	if !version.ShouldUpdate(ver.Rollout, nodeID) {
		zap.L().Info("New version available but not rolled out to this nodeID yet", zap.String("Service", updaterServiceName))
		return nil
	}

	newVersionPath := prependExtension(binaryLocation, ver.Suggested.Version)

	if err = downloadBinary(ctx, parseDownloadURL(ver.Suggested.URL), newVersionPath); err != nil {
		return errs.Wrap(err)
	}

	downloadedVersion, err := binaryVersion(newVersionPath)
	if err != nil {
		return errs.Combine(errs.Wrap(err), os.Remove(newVersionPath))
	}

	if suggestedVersion.Compare(downloadedVersion) != 0 {
		err := errs.New("invalid version downloaded: wants %s got %s",
			suggestedVersion.String(),
			downloadedVersion.String(),
		)
		return errs.Combine(err, os.Remove(newVersionPath))
	}

	zap.L().Info("Restarting service.", zap.String("Service", updaterServiceName))
	return restartSelf(binaryLocation, newVersionPath)
}

func restartSelf(bin, newbin string) error {
	args := []string{
		"restart-service",
		"--binary-location", bin,
		"--service-name", updaterServiceName,
		newbin,
	}

	return exec.Command(bin, args...).Start()
}
