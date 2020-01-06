// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/errs2"
	"storj.io/common/fpath"
	"storj.io/common/identity"
	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/process"
	"storj.io/storj/private/version"
	"storj.io/storj/private/version/checker"
)

const (
	updaterServiceName = "storagenode-updater"
	minCheckInterval   = time.Minute
)

var (
	cancel context.CancelFunc
	// TODO: replace with config value of random bytes in storagenode config.
	nodeID storj.NodeID

	rootCmd = &cobra.Command{
		Use:   "storagenode-updater",
		Short: "Version updater for storage node",
	}
	runCmd = &cobra.Command{
		Use:   "run",
		Short: "Run the storagenode-updater for storage node",
		Args:  cobra.OnlyValidArgs,
		RunE:  cmdRun,
	}

	runCfg struct {
		// TODO: check interval default has changed from 6 hours to 15 min.
		checker.Config
		Identity identity.Config

		BinaryLocation string `help:"the storage node executable binary location" default:"storagenode.exe"`
		ServiceName    string `help:"storage node OS service name" default:"storagenode"`
		// deprecated
		Log string `help:"deprecated, use --log.output" default:""`
	}

	confDir     string
	identityDir string
)

func init() {
	// TODO: this will probably generate warnings for mismatched config fields.
	defaultConfDir := fpath.ApplicationDir("storj", "storagenode")
	defaultIdentityDir := fpath.ApplicationDir("storj", "identity", "storagenode")
	cfgstruct.SetupFlag(zap.L(), rootCmd, &confDir, "config-dir", defaultConfDir, "main directory for storagenode configuration")
	cfgstruct.SetupFlag(zap.L(), rootCmd, &identityDir, "identity-dir", defaultIdentityDir, "main directory for storagenode identity credentials")
	defaults := cfgstruct.DefaultsFlag(rootCmd)

	rootCmd.AddCommand(runCmd)

	process.Bind(runCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
}

func cmdRun(cmd *cobra.Command, args []string) (err error) {
	err = openLog()
	if err != nil {
		zap.S().Errorf("Error creating new logger: %v", err)
	}

	if !fileExists(runCfg.BinaryLocation) {
		zap.S().Fatal("Unable to find storage node executable binary")
	}

	ident, err := runCfg.Identity.Load()
	if err != nil {
		zap.S().Fatalf("Error loading identity: %v", err)
	}
	nodeID = ident.ID
	if nodeID.IsZero() {
		zap.S().Fatal("Empty node ID")
	}

	var ctx context.Context
	ctx, cancel = process.Ctx(cmd)
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-c

		signal.Stop(c)
		cancel()
	}()

	loopFunc := func(ctx context.Context) (err error) {
		if err := update(ctx, runCfg.BinaryLocation, runCfg.ServiceName); err != nil {
			// don't finish loop in case of error just wait for another execution
			zap.S().Errorf("Error updating %s: %v", runCfg.ServiceName, err)
		}

		updaterBinName := os.Args[0]
		if err := update(ctx, updaterBinName, updaterServiceName); err != nil {
			// don't finish loop in case of error just wait for another execution
			zap.S().Errorf("Error updating %s: %v", updaterServiceName, err)
		}
		return nil
	}

	switch {
	case runCfg.CheckInterval <= 0:
		err = loopFunc(ctx)
	case runCfg.CheckInterval < minCheckInterval:
		zap.S().Errorf("Check interval below minimum: %s, setting to %s", runCfg.CheckInterval, minCheckInterval)
		runCfg.CheckInterval = minCheckInterval
		fallthrough
	default:
		loop := sync2.NewCycle(runCfg.CheckInterval)
		err = loop.Run(ctx, loopFunc)
	}
	if err != nil && !errs2.IsCanceled(err) {
		log.Fatal(err)
	}
	return nil
}

func update(ctx context.Context, binPath, serviceName string) (err error) {
	if nodeID.IsZero() {
		zap.S().Fatal("Empty node ID")
	}

	var currentVersion version.SemVer
	if serviceName == updaterServiceName {
		// TODO: find better way to check this binary version
		currentVersion = version.Build.Version
	} else {
		currentVersion, err = binaryVersion(binPath)
		if err != nil {
			return errs.Wrap(err)
		}
	}

	client := checker.New(runCfg.ClientConfig)
	zap.S().Infof("Downloading versions from %s", runCfg.ServerAddress)
	processVersion, err := client.Process(ctx, serviceName)
	if err != nil {
		return errs.Wrap(err)
	}

	// TODO: consolidate semver.Version and version.SemVer
	suggestedVersion, err := processVersion.Suggested.SemVer()
	if err != nil {
		return errs.Wrap(err)
	}

	if currentVersion.Compare(suggestedVersion) >= 0 {
		zap.S().Infof("%s version is up to date", serviceName)
		return nil
	}

	if !version.ShouldUpdate(processVersion.Rollout, nodeID) {
		zap.S().Infof("New %s version available but not rolled out to this nodeID yet", serviceName)
		return nil
	}

	tempArchive, err := ioutil.TempFile("", serviceName)
	if err != nil {
		return errs.New("cannot create temporary archive: %v", err)
	}
	defer func() {
		err = errs.Combine(err,
			tempArchive.Close(),
			os.Remove(tempArchive.Name()),
		)
	}()

	downloadURL := parseDownloadURL(processVersion.Suggested.URL)
	zap.S().Infof("Start downloading %s to %s", downloadURL, tempArchive.Name())
	err = downloadArchive(ctx, tempArchive, downloadURL)
	if err != nil {
		return errs.Wrap(err)
	}
	zap.S().Infof("Finished downloading %s to %s", downloadURL, tempArchive.Name())

	newVersionPath := prependExtension(binPath, suggestedVersion.String())
	err = unpackBinary(ctx, tempArchive.Name(), newVersionPath)
	if err != nil {
		return errs.Wrap(err)
	}

	// TODO add here recovery even before starting service (if version command cannot be executed)

	downloadedVersion, err := binaryVersion(newVersionPath)
	if err != nil {
		return errs.Wrap(err)
	}

	if suggestedVersion.Compare(downloadedVersion) != 0 {
		return errs.New("invalid version downloaded: wants %s got %s", suggestedVersion.String(), downloadedVersion.String())
	}

	// backup original binary
	var backupPath string
	if serviceName == updaterServiceName {
		// NB: don't include old version number for updater binary backup
		backupPath = prependExtension(binPath, "old")
	} else {
		backupPath = prependExtension(binPath, "old."+currentVersion.String())
	}
	if err := os.Rename(binPath, backupPath); err != nil {
		return errs.Wrap(err)
	}

	// rename new binary to replace original
	if err := os.Rename(newVersionPath, binPath); err != nil {
		return errs.Wrap(err)
	}

	zap.S().Infof("Restarting service %s", serviceName)
	err = restartService(serviceName)
	if err != nil {
		// TODO: should we try to recover from this?
		return errs.New("Unable to restart service: %v", err)
	}
	zap.S().Infof("Service %s restarted successfully", serviceName)

	// TODO remove old binary ??
	return nil
}

func prependExtension(path, ext string) string {
	originalExt := filepath.Ext(path)
	dir, base := filepath.Split(path)
	base = base[:len(base)-len(originalExt)]
	return filepath.Join(dir, base+"."+ext+originalExt)
}

func parseDownloadURL(template string) string {
	url := strings.Replace(template, "{os}", runtime.GOOS, 1)
	url = strings.Replace(url, "{arch}", runtime.GOARCH, 1)
	return url
}

func binaryVersion(location string) (version.SemVer, error) {
	out, err := exec.Command(location, "version").CombinedOutput()
	if err != nil {
		zap.S().Infof("Command output: %s", out)
		return version.SemVer{}, err
	}

	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		prefix := "Version: "
		if strings.HasPrefix(line, prefix) {
			line = line[len(prefix):]
			return version.NewSemVer(line)
		}
	}
	return version.SemVer{}, errs.New("unable to determine binary version")
}

func downloadArchive(ctx context.Context, file io.Writer, url string) (err error) {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}

	defer func() { err = errs.Combine(err, resp.Body.Close()) }()

	if resp.StatusCode != http.StatusOK {
		return errs.New("bad status: %s", resp.Status)
	}

	_, err = sync2.Copy(ctx, file, resp.Body)
	return err
}

func unpackBinary(ctx context.Context, archive, target string) (err error) {
	// TODO support different compression types e.g. tar.gz

	zipReader, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer func() { err = errs.Combine(err, zipReader.Close()) }()

	if len(zipReader.File) != 1 {
		return errors.New("archive should contain only binary file")
	}

	zipedExec, err := zipReader.File[0].Open()
	if err != nil {
		return err
	}
	defer func() { err = errs.Combine(err, zipedExec.Close()) }()

	newExec, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, os.FileMode(0755))
	if err != nil {
		return err
	}
	defer func() { err = errs.Combine(err, newExec.Close()) }()

	_, err = sync2.Copy(ctx, newExec, zipedExec)
	if err != nil {
		return errs.Combine(err, os.Remove(newExec.Name()))
	}
	return nil
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return info.Mode().IsRegular()
}

func openLog() error {
	if runCfg.Log != "" {
		logPath := runCfg.Log
		if runtime.GOOS == "windows" && !strings.HasPrefix(logPath, "winfile:///") {
			logPath = "winfile:///" + logPath
		}
		logger, err := process.NewLoggerWithOutputPaths(logPath)
		if err != nil {
			return err
		}
		zap.ReplaceGlobals(logger)
	}
	return nil
}

func main() {
	process.Exec(rootCmd)
}
