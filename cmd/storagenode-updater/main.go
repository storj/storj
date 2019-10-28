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

	"storj.io/storj/internal/errs2"
	"storj.io/storj/internal/fpath"
	"storj.io/storj/internal/sync2"
	"storj.io/storj/internal/version"
	"storj.io/storj/internal/version/checker"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/storj"
)

const minCheckInterval = time.Minute

var (
	cancel context.CancelFunc

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
		Log            string `help:"path to log file, if empty standard output will be used" default:""`
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
	if runCfg.Log != "" {
		logFile, err := os.OpenFile(runCfg.Log, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			log.Fatalf("error opening log file: %s", err)
		}
		defer func() { err = errs.Combine(err, logFile.Close()) }()
		log.SetOutput(logFile)
	}

	if !fileExists(runCfg.BinaryLocation) {
		log.Fatal("unable to find storage node executable binary")
	}

	ident, err := runCfg.Identity.Load()
	if err != nil {
		log.Fatalf("error loading identity: %s", err)
	}

	var ctx context.Context
	ctx, cancel = context.WithCancel(context.Background())
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-c

		signal.Stop(c)
		cancel()
	}()

	loopFunc := func(ctx context.Context) (err error) {
		if err := update(ctx, ident.ID); err != nil {
			// don't finish loop in case of error just wait for another execution
			log.Println(err)
		}
		return nil
	}

	switch {
	case runCfg.CheckInterval <= 0:
		err = loopFunc(ctx)
	case runCfg.CheckInterval < minCheckInterval:
		log.Printf("check interval below minimum: \"%s\", setting to %s", runCfg.CheckInterval, minCheckInterval)
		runCfg.CheckInterval = minCheckInterval
		fallthrough
	default:
		loop := sync2.NewCycle(runCfg.CheckInterval)
		err = loop.Run(ctx, loopFunc)
	}
	if err != nil && errs2.IsCanceled(err) {
		log.Fatal(err)
	}
	return nil
}

func update(ctx context.Context, nodeID storj.NodeID) (err error) {
	client := checker.New(runCfg.ClientConfig)

	currentVersion, err := binaryVersion(runCfg.BinaryLocation)
	if err != nil {
		return errs.Wrap(err)
	}

	log.Println("downloading versions from", runCfg.ServerAddress)
	shouldUpdate, newVersion, err := client.ShouldUpdate(ctx, runCfg.ServiceName, nodeID)
	if err != nil {
		return errs.Wrap(err)
	}

	if shouldUpdate {
		downloadURL := newVersion.URL
		downloadURL = strings.Replace(downloadURL, "{os}", runtime.GOOS, 1)
		downloadURL = strings.Replace(downloadURL, "{arch}", runtime.GOARCH, 1)
		// TODO: consolidate semver.Version and version.SemVer
		suggestedVersion, err := newVersion.SemVer()
		if err != nil {
			return errs.Wrap(err)
		}

		if currentVersion.Compare(suggestedVersion) < 0 {
			tempArchive, err := ioutil.TempFile(os.TempDir(), runCfg.ServiceName)
			if err != nil {
				return errs.New("cannot create temporary archive: %v", err)
			}
			defer func() { err = errs.Combine(err, os.Remove(tempArchive.Name())) }()

			log.Println("start downloading", downloadURL, "to", tempArchive.Name())
			err = downloadArchive(ctx, tempArchive, downloadURL)
			if err != nil {
				return errs.Wrap(err)
			}
			log.Println("finished downloading", downloadURL, "to", tempArchive.Name())

			extension := filepath.Ext(runCfg.BinaryLocation)
			if extension != "" {
				extension = "." + extension
			}

			dir := filepath.Dir(runCfg.BinaryLocation)
			backupExec := filepath.Join(dir, runCfg.ServiceName+".old."+currentVersion.String()+extension)

			if err = os.Rename(runCfg.BinaryLocation, backupExec); err != nil {
				return errs.Wrap(err)
			}

			err = unpackBinary(ctx, tempArchive.Name(), runCfg.BinaryLocation)
			if err != nil {
				return errs.Wrap(err)
			}

			downloadedVersion, err := binaryVersion(runCfg.BinaryLocation)
			if err != nil {
				return errs.Wrap(err)
			}

			if suggestedVersion.Compare(downloadedVersion) != 0 {
				return errs.New("invalid version downloaded: wants %s got %s", suggestedVersion.String(), downloadedVersion.String())
			}

			log.Println("restarting service", runCfg.ServiceName)
			err = restartSNService(runCfg.ServiceName)
			if err != nil {
				// TODO: should we try to recover from this?
				return errs.New("unable to restart service: %v", err)
			}
			log.Println("service", runCfg.ServiceName, "restarted successfully")

			// TODO remove old binary ??
		} else {
			log.Printf("%s version is up to date\n", runCfg.ServiceName)
		}
	}

	return nil
}

func binaryVersion(location string) (version.SemVer, error) {
	out, err := exec.Command(location, "version").Output()
	if err != nil {
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

func restartSNService(name string) error {
	switch runtime.GOOS {
	case "windows":
		// TODO how run this as one command `net stop servicename && net start servicename`?
		// TODO: combine stdout with err if err
		_, err := exec.Command("net", "stop", name).CombinedOutput()
		if err != nil {
			return err
		}
		_, err = exec.Command("net", "start", name).CombinedOutput()
		if err != nil {
			return err
		}
	default:
		return nil
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

func main() {
	_ = rootCmd.Execute()
}
