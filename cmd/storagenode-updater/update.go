// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"os"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/private/version"
)

func update(ctx context.Context, serviceName, binaryLocation string, ver version.Process) error {
	suggestedVersion, err := ver.Suggested.SemVer()
	if err != nil {
		return errs.Wrap(err)
	}

	var currentVersion version.SemVer
	if serviceName == updaterServiceName {
		currentVersion = version.Build.Version
	} else {
		currentVersion, err = binaryVersion(binaryLocation)
		if err != nil {
			return errs.Wrap(err)
		}
	}

	// should update
	if currentVersion.Compare(suggestedVersion) >= 0 {
		zap.L().Info("Version is up to date.", zap.String("Service", serviceName))
		return nil
	}
	if !version.ShouldUpdate(ver.Rollout, nodeID) {
		zap.L().Info("New version available but not rolled out to this nodeID yet", zap.String("Service", serviceName))
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

	var backupPath string
	if serviceName == updaterServiceName {
		// NB: don't include old version number for updater binary backup
		backupPath = prependExtension(binaryLocation, "old")
	} else {
		backupPath = prependExtension(binaryLocation, "old."+currentVersion.String())
	}

	zap.L().Info("Restarting service.", zap.String("Service", serviceName))

	if err = restartService(ctx, serviceName, binaryLocation, newVersionPath, backupPath); err != nil {
		return errs.Wrap(err)
	}

	zap.L().Info("Service restarted successfully.", zap.String("Service", serviceName))
	return nil
}
