// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// +build windows
// +build ignore

package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"

	"storj.io/storj/private/testcontext"
)

const (
	// TODO: make this more dynamic and/or use versioncontrol server?
	// (NB: can't use versioncontrol server until updater process is added to response)
	oldRelease = "v0.26.2"

	testWalletAddr = "0x0000000000000000000000000000000000000000"
	testEmail      = "user@mail.test"
	testPublicAddr = "127.0.0.1:10000"
)

var (
	releaseMSIPath string

	msiBaseArgs = []string{
		"/quiet", "/qn",
		"/norestart",
	}
	serviceNames = []string{"storagenode", "storagenode-updater"}

	msiPathFlag = flag.String("msi", "", "path to the msi to use")
)

func TestMain(m *testing.M) {
	flag.Parse()
	if *msiPathFlag == "" {
		log.Fatal("no msi passed, use `go test ... -args -msi \"<msi path (using \\ seperator)>\"`")
	}

	tmp, err := ioutil.TempDir("", "release")
	if err != nil {
		panic(err)
	}

	releaseMSIPath = filepath.Join(tmp, oldRelease+".msi")
	if err = downloadInstaller(oldRelease, releaseMSIPath); err != nil {
		panic(err)
	}
	defer func() {
		err = errs.Combine(os.RemoveAll(tmp))
	}()

	viper.SetConfigType("yaml")

	os.Exit(m.Run())
}

func TestService_StopStart(t *testing.T) {
	ctx := testcontext.NewWithTimeout(t, 30*time.Second)
	defer ctx.Cleanup()

	installDir := ctx.Dir("install")
	testInstall(t, ctx, *msiPathFlag, installDir)
	defer requireUninstall(t, ctx)

	// NB: wait for install to complete / services to be running
	noop := func(*mgr.Service) error { return nil }
	err := controlServices(ctx, svc.Running, noop)
	require.NoError(t, err)

	require.NoError(t, stopServices(ctx))
	require.NoError(t, startServices(ctx))
}

func TestInstaller_Config(t *testing.T) {
	ctx := testcontext.NewWithTimeout(t, 30*time.Second)
	defer ctx.Cleanup()

	installDir := ctx.Dir("install")
	testInstall(t, ctx, *msiPathFlag, installDir)
	defer requireUninstall(t, ctx)

	configFile, err := os.Open(ctx.File("install", "config.yaml"))
	require.NoError(t, err)
	defer ctx.Check(configFile.Close)

	err = viper.ReadConfig(configFile)
	require.NoError(t, err)

	require.Equal(t, testEmail, viper.GetString("operator.email"))
	require.Equal(t, testWalletAddr, viper.GetString("operator.wallet"))
	require.Equal(t, testPublicAddr, viper.GetString("contact.external-address"))
}

func TestUpgrade_Config(t *testing.T) {
	if *msiPathFlag == "" {
		t.Fatal("no msi passed, use `go test ... -args -msi <msi path>`")
	}

	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	installDir := ctx.Dir("install")
	testInstall(t, ctx, releaseMSIPath, installDir)

	// upgrade using test msi
	install(t, ctx, *msiPathFlag, "")
	defer requireUninstall(t, ctx)

	configFile, err := os.Open(ctx.File("install", "config.yaml"))
	require.NoError(t, err)
	defer ctx.Check(configFile.Close)

	err = viper.ReadConfig(configFile)
	require.NoError(t, err)

	require.Equal(t, testEmail, viper.GetString("operator.email"))
	require.Equal(t, testWalletAddr, viper.GetString("operator.wallet"))
	require.Equal(t, testPublicAddr, viper.GetString("contact.external-address"))
}

func testInstall(t *testing.T, ctx *testcontext.Context, msiPath, installDir string) {
	args := []string{
		fmt.Sprintf("STORJ_WALLET=%s", testWalletAddr),
		fmt.Sprintf("STORJ_EMAIL=%s", testEmail),
		fmt.Sprintf("STORJ_PUBLIC_ADDRESS=%s", testPublicAddr),
	}
	install(t, ctx, msiPath, installDir, args...)
}

func install(t *testing.T, ctx *testcontext.Context, msiPath, installDir string, args ...string) {
	t.Logf("installing from %s\n", msiPath)
	logPath := ctx.File("log", "install.log")

	baseArgs := msiBaseArgs
	if installDir != "" {
		baseArgs = append(baseArgs, "INSTALLFOLDER="+installDir)
	}
	args = append(append([]string{
		"/i", msiPath,
		"/log", logPath,
	}, baseArgs...), args...)

	installOut, err := exec.Command("msiexec", args...).CombinedOutput()
	if !assert.NoError(t, err) {
		installLogData, err := ioutil.ReadFile(logPath)
		if assert.NoError(t, err) {
			t.Logf("MSIExec log:\n============================\n%s", string(installLogData))
		}
		t.Logf("MSIExec output:\n============================\n%s", string(installOut))
		t.Fatal()
	}
}

func requireUninstall(t *testing.T, ctx *testcontext.Context) {
	logPath := ctx.File("log", "uninstall.log")
	uninstallOut, err := uninstall(ctx, logPath)
	if !assert.NoError(t, err) {
		uninstallLogData, err := ioutil.ReadFile(logPath)
		if !assert.NoError(t, err) {
			t.Logf("MSIExec log:\n============================\n%s\n", string(uninstallLogData))
		}
		t.Fatalf("MSIExec output:\n============================\n%s", string(uninstallOut))
	}
}

func uninstall(ctx context.Context, logPath string) ([]byte, error) {
	if err := stopServices(ctx); err != nil {
		return []byte{}, err
	}

	args := append([]string{"/uninstall", *msiPathFlag, "/log", logPath}, msiBaseArgs...)
	return exec.Command("msiexec", args...).CombinedOutput()
}

func startServices(ctx context.Context, names ...string) (err error) {
	return controlServices(ctx, svc.Running, func(service *mgr.Service) error {
		return service.Start()
	})
}

func stopServices(ctx context.Context) (err error) {
	return controlServices(ctx, svc.Stopped, func(service *mgr.Service) error {
		_, err := service.Control(svc.Stop)
		return err
	})
}

func controlServices(ctx context.Context, waitState svc.State, controlF func(*mgr.Service) error) error {
	serviceMgr, err := mgr.Connect()
	if err != nil {
		return err
	}

	group := new(errgroup.Group)
	for _, name := range serviceNames {
		name := name
		group.Go(func() (err error) {
			service, err := serviceMgr.OpenService(name)
			if err != nil {
				return err
			}
			defer func() {
				err = errs.Combine(err, service.Close())
			}()

			if err := controlF(service); err != nil {
				return err
			}
			return waitForState(ctx, service, waitState)
		})
	}

	return group.Wait()
}

func waitForState(ctx context.Context, service *mgr.Service, state svc.State) error {
	for {
		status, err := service.Query()
		if err != nil {
			return err
		}

		if err := ctx.Err(); err != nil {
			return err
		}

		switch status.State {
		case state:
			return nil
		default:
			time.Sleep(500 * time.Millisecond)
		}
	}
}

func readConfigLines(t *testing.T, ctx *testcontext.Context, configPath string) string {
	configFile, err := os.Open(configPath)
	require.NoError(t, err)
	defer ctx.Check(configFile.Close)

	// NB: strip empty lines and comments
	configBuf := new(bytes.Buffer)
	scanner := bufio.NewScanner(configFile)
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.Trim(line, " \t\n")
		out := append(scanner.Bytes(), byte('\n'))
		if len(line) == 0 {
			continue
		}
		if !strings.HasPrefix(line, "#") {
			_, err := configBuf.Write(out)
			require.NoError(t, err)
		}
	}
	if err := scanner.Err(); err != nil {
		require.NoError(t, err)
	}
	return configBuf.String()
}

func releaseUrl(version, name, ext string) string {
	urlTemplate := "https://github.com/storj/storj/releases/download/{version}/{service}_{os}_{arch}{ext}.zip"

	url := strings.Replace(urlTemplate, "{version}", version, 1)
	url = strings.Replace(url, "{service}", name, 1)
	url = strings.Replace(url, "{os}", runtime.GOOS, 1)
	url = strings.Replace(url, "{arch}", runtime.GOARCH, 1)
	url = strings.Replace(url, "{ext}", ext, 1)
	return url
}

func downloadInstaller(version, dst string) error {
	zipDir, err := ioutil.TempDir("", "archive")
	if err != nil {
		return err
	}
	zipPath := filepath.Join(zipDir, version+".msi.zip")

	url := releaseUrl(version, "storagenode", ".msi")

	if err := downloadArchive(url, zipPath); err != nil {
		return err
	}
	if err := unpackBinary(zipPath, releaseMSIPath); err != nil {
		return err
	}
	return nil
}

func downloadArchive(url, dst string) (err error) {
	resp, err := http.Get(url)
	defer func() {
		err = errs.Combine(err, resp.Body.Close())
	}()

	if resp.StatusCode != http.StatusOK {
		return errs.New("error downloading %s; non-success status: %s", url, resp.Status)
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	defer func() {
		err = errs.Combine(err, dstFile.Close())
	}()

	if _, err = io.Copy(dstFile, resp.Body); err != nil {
		return err
	}
	return nil
}

func unpackBinary(archive, dst string) (err error) {
	zipReader, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer func() {
		err = errs.Combine(err, zipReader.Close())
	}()

	zipedExec, err := zipReader.File[0].Open()
	if err != nil {
		return err
	}
	defer func() {
		err = errs.Combine(err, zipedExec.Close())
	}()

	newExec, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	defer func() {
		err = errs.Combine(err, newExec.Close())
	}()

	_, err = io.Copy(newExec, zipedExec)
	if err != nil {
		return err
	}
	return nil
}
