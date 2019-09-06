// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"

	"storj.io/storj/internal/sync2"
	"storj.io/storj/internal/version"
)

var (
	rootCmd = &cobra.Command{
		Use:   "auto-updater",
		Short: "Auto-updater for storage node",
	}
	runCmd = &cobra.Command{
		Use:   "run",
		Short: "Run the auto updater for storage node",
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
)

// Response response from version server.
type Response struct {
	Processes Processes `json:"processes"`
}

// Processes describes versions for each binary.
type Processes struct {
	Storagenode Process `json:"storagenode"`
}

// Process versions for specific binary.
type Process struct {
	Minimum   Version `json:"minimum"`
	Suggested Version `json:"suggested"`
}

// Version represents version and download URL for binary.
type Version struct {
	Version string `json:"version"`
	URL     string `json:"url"`
}

func init() {
	rootCmd.AddCommand(runCmd)

	runCmd.Flags().StringVar(&interval, "interval", "06h", "interval for checking the new version")
	runCmd.Flags().StringVar(&versionURL, "version-url", "https://version.storj.io/release/", "version server URL")
	runCmd.Flags().StringVar(&binaryLocation, "binary-location", "storagenode.exe", "the storage node executable binary location")
}

func cmdRun(cmd *cobra.Command, args []string) (err error) {
	if !fileExists(binaryLocation) {
		return errs.New("unable to find storage node executable binary")
	}

	ctx, cancel := context.WithCancel(context.Background())
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-c

		signal.Stop(c)
		cancel()
	}()

	loopInterval, err := time.ParseDuration(interval)
	if err != nil {
		return errs.New("unable to parse interval parameter: %v", err)
	}

	loop := sync2.NewCycle(loopInterval)

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

			// TODO next PRs
			// * rename current binary into `storagenode.old.<release>.exe`.
			// * unzip downloaded binary in place of current binary
			// * compare extracted binary with version from suggested version
			// * restart storage node service
		}
		return nil
	}

	err = loop.Run(ctx, func(ctx context.Context) (err error) {
		if err := update(ctx); err != nil {
			// don't finish loop in case of error just wait for another execution
			log.Println(err)
		}
		return nil
	})
	if err != context.Canceled {
		return err
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
			return version.NewSemVer(line[len(prefix):])
		}
	}
	return version.SemVer{}, errs.New("unable to determine binary version")
}

func suggestedVersion() (ver version.SemVer, url string, err error) {
	resp, err := http.Get(versionURL)
	if err != nil {
		return ver, url, err
	}
	defer func() { err = errs.Combine(err, resp.Body.Close()) }()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return ver, url, err
	}

	var response Response
	err = json.Unmarshal(body, &response)
	if err != nil {
		return ver, url, err
	}

	suggestedVersion := response.Processes.Storagenode.Suggested
	ver, err = version.NewSemVer(suggestedVersion.Version)
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
