// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

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
	newSemVer, err := newVersion.SemVer()
	if err != nil {
		return err
	}

	// do not try to redownload the binary that failed to start
	if lastFailure, ok := loadLastUpdateFailure(ctx, log, storeDir, serviceName); ok {
		if lastFailure.Version.Compare(newSemVer) == 0 {
			return nil
		}
	}

	newVersionPath := prependExtension(binaryLocation, newVersion.Version)

	if err = downloadBinary(ctx, parseDownloadURL(newVersion.URL), newVersionPath); err != nil {
		return errs.Wrap(err)
	}

	downloadedVersion, err := binaryVersion(newVersionPath)
	if err != nil {
		return errs.Combine(errs.Wrap(err), os.Remove(newVersionPath))
	}

	if newSemVer.Compare(downloadedVersion) != 0 {
		err := errs.New("invalid version downloaded: wants %s got %s", newVersion.Version, downloadedVersion)
		return errs.Combine(err, os.Remove(newVersionPath))
	}

	if err := tryRunBinary(ctx, log, serviceName, newVersionPath); err != nil {
		saveLastUpdateFailure(ctx, log, storeDir, serviceName, failedUpdate{
			Version: newSemVer,
			Date:    time.Now(),
			Failure: err.Error(),
		})
		return errs.Combine(
			errs.New("unable to run binary: %w", err),
			os.Remove(newVersionPath),
		)
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
	exit, err := swapBinariesAndRestart(ctx, restartMethod, service, binaryLocation, newVersionPath, backupPath)
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

// tryRunBinary tries to execute the binary to see whether basic functionality works on the current system.
func tryRunBinary(ctx context.Context, log *zap.Logger, serviceName, binaryLocation string) error {
	cmd := exec.Command(binaryLocation, "--help")
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Error("failed to run binary", zap.String("output", string(out)))
		return errs.New("%w\n%s", err, out)
	}
	return nil
}

type failedUpdate struct {
	Version version.SemVer
	Date    time.Time
	Failure string
}

// lastFailedUpdateName returns the json file where we store update failure.
func lastFailedUpdateName(storeDir, serviceName string) string {
	return filepath.Join(storeDir, "last-failed-update."+serviceName+".json")
}

// loadLastUpdateFailure loads information about the last failed update.
func loadLastUpdateFailure(ctx context.Context, log *zap.Logger, storeDir, serviceName string) (_ failedUpdate, ok bool) {
	log = log.Named("log-failure")

	name := lastFailedUpdateName(storeDir, serviceName)
	data, err := os.ReadFile(name)
	if err != nil {
		if os.IsNotExist(err) {
			return failedUpdate{}, false
		}
		log.Error("failed to read update failure file", zap.Error(err))
		return failedUpdate{}, false
	}

	var result failedUpdate
	err = json.Unmarshal(data, &result)
	if err != nil {
		log.Error("failed to read update failure file", zap.Error(err))
		return failedUpdate{}, false
	}

	return result, true
}

// saveLastUpdateFailure writes information about the last failed update.
func saveLastUpdateFailure(ctx context.Context, log *zap.Logger, storeDir, serviceName string, fail failedUpdate) {
	log = log.Named("log-failure")

	name := lastFailedUpdateName(storeDir, serviceName)
	data, err := json.MarshalIndent(fail, "", "    ")
	if err != nil {
		log.Error("failed to marshal json for update failure", zap.Error(err))
		return
	}

	err = os.WriteFile(name, data, 0755)
	if err != nil {
		log.Error("failed to write json for update failure", zap.Error(err))
		return
	}
}
