// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/version"
)

func update(ctx context.Context, restartMethod, serviceName, binaryLocation, storeDir string, ver version.Process) error {
	currentVersion, err := binaryVersion(binaryLocation)
	if err != nil {
		return errs.Wrap(err)
	}

	log := zap.L().With(zap.String("Service", serviceName))

	log.Info("Current binary version",
		zap.String("Version", currentVersion.String()),
	)

	// should update
	newVersion, reason, err := version.ShouldUpdateVersion(currentVersion, nodeID, ver)
	if err != nil {
		return errs.Wrap(err)
	}
	if newVersion.IsZero() {
		log.Info(reason)
		return nil
	}

	newVersionPath := prependExtension(binaryLocation, newVersion.Version)

	if err = downloadBinary(ctx, parseDownloadURL(newVersion.URL), newVersionPath); err != nil {
		return errs.Wrap(err)
	}

	downloadedVersion, err := binaryVersion(newVersionPath)
	if err != nil {
		return errs.Combine(errs.Wrap(err), os.Remove(newVersionPath))
	}

	newSemVer, err := newVersion.SemVer()
	if err != nil {
		return errs.Combine(err, os.Remove(newVersionPath))
	}

	if newSemVer.Compare(downloadedVersion) != 0 {
		err := errs.New("invalid version downloaded: wants %s got %s", newVersion.Version, downloadedVersion)
		return errs.Combine(err, os.Remove(newVersionPath))
	}

	if err = copyToStore(binaryLocation, storeDir); err != nil {
		return errs.Wrap(err)
	}

	var backupPath string
	if serviceName == updaterServiceName {
		// NB: don't include old version number for updater binary backup
		backupPath = prependExtension(binaryLocation, "old")
	} else {
		backupPath = prependExtension(binaryLocation, "old."+currentVersion.String())
	}

	if err = restartAndCleanup(ctx, log, restartMethod, serviceName, binaryLocation, newVersionPath, backupPath); err != nil {
		return errs.Wrap(err)
	}
	return nil
}

// copyToStore copies binary to store directory if the storeDir is set and different from the binary location.
func copyToStore(binaryLocation, storeDir string) error {
	if storeDir == "" {
		return nil
	}

	dir, base := filepath.Split(binaryLocation)
	if dir == storeDir {
		return nil
	}

	storeLocation := filepath.Join(storeDir, base)

	log := zap.L().With(zap.String("Service", "copyToStore"))

	// copy binary to store
	log.Info("Copying binary to store.", zap.String("From", binaryLocation), zap.String("To", storeLocation))
	src, err := os.Open(binaryLocation)
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() {
		err = errs.Combine(err, src.Close())
	}()

	dest, err := os.OpenFile(storeLocation, os.O_WRONLY|os.O_CREATE, 0755)
	if err != nil {
		return errs.Wrap(err)
	}

	defer func() {
		err = errs.Combine(err, dest.Close())
	}()

	_, err = io.Copy(dest, src)
	if err != nil {
		return errs.Wrap(err)
	}

	log.Info("Binary copied to store.", zap.String("From", binaryLocation), zap.String("To", storeLocation))

	return nil
}

func restartAndCleanup(ctx context.Context, log *zap.Logger, restartMethod, service, binaryLocation, newVersionPath, backupPath string) error {
	log.Info("Restarting service.")
	exit, err := restartService(ctx, restartMethod, service, binaryLocation, newVersionPath, backupPath)
	if err != nil {
		return err
	}

	if !exit {
		log.Info("Service restarted successfully.")
	}

	log.Info("Cleaning up old binary.", zap.String("Path", backupPath))
	if err := os.Remove(backupPath); err != nil && !errs.Is(err, os.ErrNotExist) {
		log.Error("Failed to remove backup binary. Consider removing manually.", zap.String("Path", backupPath), zap.Error(err))
	}

	if exit {
		os.Exit(1)
	}

	return nil
}
