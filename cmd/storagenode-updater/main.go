// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

	"github.com/blang/semver"
	"github.com/spf13/cobra"
	"github.com/zeebo/errs"

	"storj.io/storj/internal/sync2"
	"storj.io/storj/internal/version"
)

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
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			err = cmdRun(cmd, args)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			return nil
		},
	}

	interval       string
	versionURL     string
	binaryLocation string
	snServiceName  string
	logPath        string
)

func init() {
	rootCmd.AddCommand(runCmd)

	runCmd.Flags().StringVar(&interval, "interval", "06h", "interval for checking the new version, 0 or less value will execute version check only once")
	runCmd.Flags().StringVar(&versionURL, "version-url", "https://version.storj.io/release/", "version server URL")
	runCmd.Flags().StringVar(&binaryLocation, "binary-location", "storagenode.exe", "the storage node executable binary location")

	runCmd.Flags().StringVar(&snServiceName, "service-name", "storagenode", "storage node OS service name")
	runCmd.Flags().StringVar(&logPath, "log", "", "path to log file, if empty standard output will be used")
}

func cmdRun(cmd *cobra.Command, args []string) (err error) {
	if logPath != "" {
		logFile, err := os.OpenFile(logPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			return errs.New("error opening log file: %v", err)
		}
		defer func() { err = errs.Combine(err, logFile.Close()) }()
		log.SetOutput(logFile)
	}

	if !fileExists(binaryLocation) {
		return errs.New("unable to find storage node executable binary")
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

	update := func(ctx context.Context) (err error) {
		currentVersion, err := binaryVersion(binaryLocation)
		if err != nil {
			return err
		}
		log.Println("downloading versions from", versionURL)
		suggestedVersion, downloadURL, err := suggestedVersion()
		if err != nil {
			return err
		}

		downloadURL = strings.Replace(downloadURL, "{os}", runtime.GOOS, 1)
		downloadURL = strings.Replace(downloadURL, "{arch}", runtime.GOARCH, 1)

		if currentVersion.Compare(suggestedVersion) < 0 {
			tempArchive, err := ioutil.TempFile(os.TempDir(), "storagenode")
			if err != nil {
				return errs.New("cannot create temporary archive: %v", err)
			}
			defer func() { err = errs.Combine(err, os.Remove(tempArchive.Name())) }()

			log.Println("start downloading", downloadURL, "to", tempArchive.Name())
			err = downloadArchive(ctx, tempArchive, downloadURL)
			if err != nil {
				return err
			}
			log.Println("finished downloading", downloadURL, "to", tempArchive.Name())

			extension := filepath.Ext(binaryLocation)
			if extension != "" {
				extension = "." + extension
			}

			dir := filepath.Dir(binaryLocation)
			backupExec := filepath.Join(dir, "storagenode.old."+currentVersion.String()+extension)

			if err = os.Rename(binaryLocation, backupExec); err != nil {
				return err
			}

			err = unpackBinary(ctx, tempArchive.Name(), binaryLocation)
			if err != nil {
				return err
			}

			downloadedVersion, err := binaryVersion(binaryLocation)
			if err != nil {
				return err
			}

			if suggestedVersion.Compare(downloadedVersion) != 0 {
				return errs.New("invalid version downloaded: wants %s got %s", suggestedVersion.String(), downloadedVersion.String())
			}

			log.Println("restarting service", snServiceName)
			err = restartSNService(snServiceName)
			if err != nil {
				return errs.New("unable to restart service: %v", err)
			}
			log.Println("service", snServiceName, "restarted successfully")

			// TODO remove old binary ??
		} else {
			log.Println("storage node version is up to date")
		}
		return nil
	}

	loopInterval, err := time.ParseDuration(interval)
	if err != nil {
		return errs.New("unable to parse interval parameter: %v", err)
	}

	loopFunc := func(ctx context.Context) (err error) {
		if err := update(ctx); err != nil {
			// don't finish loop in case of error just wait for another execution
			log.Println(err)
		}
		return nil
	}

	if loopInterval <= 0 {
		err = loopFunc(ctx)
	} else {
		loop := sync2.NewCycle(loopInterval)
		err = loop.Run(ctx, loopFunc)
	}
	if err != context.Canceled {
		return err
	}
	return nil
}

func binaryVersion(location string) (semver.Version, error) {
	out, err := exec.Command(location, "version").Output()
	if err != nil {
		return semver.Version{}, err
	}

	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		prefix := "Version: "
		if strings.HasPrefix(line, prefix) {
			line = line[len(prefix):]
			if strings.HasPrefix(line, "v") {
				line = line[1:]
			}
			return semver.Make(line)
		}
	}
	return semver.Version{}, errs.New("unable to determine binary version")
}

func suggestedVersion() (ver semver.Version, url string, err error) {
	resp, err := http.Get(versionURL)
	if err != nil {
		return ver, url, err
	}
	defer func() { err = errs.Combine(err, resp.Body.Close()) }()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return ver, url, err
	}

	var response version.AllowedVersions
	err = json.Unmarshal(body, &response)
	if err != nil {
		return ver, url, err
	}

	suggestedVersion := response.Processes.Storagenode.Suggested
	ver, err = semver.Make(suggestedVersion.Version)
	if err != nil {
		return ver, url, err
	}
	return ver, suggestedVersion.URL, nil
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
		_, err := exec.Command("net", "stop", name).Output()
		if err != nil {
			return err
		}
		_, err = exec.Command("net", "start", name).Output()
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
