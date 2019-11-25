// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// +build ignore

package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"

	"storj.io/storj/private/sync2"
	"storj.io/storj/private/testcontext"
)

var (
	installerDir   string
	storagenodeBin string
	updaterBin     string
	builtMSIPath   string
	releaseMSIPath string

	// TODO: make this more dynamic and/or use versioncontrol server?
	// (NB: can't use versioncontrol server until updater process is added to response)
	downloadVersion    = "v0.26.2"
	buildInstallerOnce = sync.Once{}
	msiBaseArgs        = []string{
		"/passive", "/qb",
		"/norestart",
	}
)

func TestMain(m *testing.M) {
	var err error
	installerDir, err = filepath.Abs(filepath.Join("..", "installer", "windows"))
	if err != nil {
		panic(err)
	}

	storagenodeBin = filepath.Join(installerDir, "storagenode.exe")
	updaterBin = filepath.Join(installerDir, "storagenode-updater.exe")

	tmp, err := ioutil.TempDir("", "installer")
	if err != nil {
		panic(err)
	}
	releaseMSIPath = filepath.Join(tmp, "installer.msi")

	msiDir := filepath.Join(installerDir, "bin", "Release")
	builtMSIPath = filepath.Join(msiDir, "storagenode.msi")

	status := m.Run()

	err = os.Remove(storagenodeBin)
	if err != nil && !os.IsNotExist(err) {
		log.Printf("unable to cleanup temp storagenode binary at \"%s\": %s", storagenodeBin, err)
	}
	err = os.Remove(updaterBin)
	if err != nil && !os.IsNotExist(err) {
		log.Printf("unable to cleanup temp updater binary at \"%s\": %s", updaterBin, err)
	}
	err = os.Remove(releaseMSIPath)
	if err != nil && !os.IsNotExist(err) {
		log.Printf("unable to cleanup temp relase installer at \"%s\": %s", releaseMSIPath, err)
	}

	os.Exit(status)
}

func TestInstaller_Config(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	downloadInstaller(t, ctx)
	tryUninstall(t, ctx, releaseMSIPath)

	installDir := ctx.Dir("install")
	configPath := ctx.File("install", "config.yaml")

	walletAddr := "0x0000000000000000000000000000000000000000"
	email := "user@mail.test"
	publicAddr := "127.0.0.1:10000"

	args := []string{
		fmt.Sprintf("INSTALLFOLDER=%s", installDir),
		//fmt.Sprintf("STORJ_IDENTITYDIR=%s", installDir),
		fmt.Sprintf("STORJ_WALLET=%s", walletAddr),
		fmt.Sprintf("STORJ_EMAIL=%s", email),
		fmt.Sprintf("STORJ_PUBLIC_ADDRESS=%s", publicAddr),
	}
	install(t, ctx, releaseMSIPath, args...)
	defer requireUninstall(t, ctx, releaseMSIPath)
	defer ctx.Check(func() error {
		return stopServices("storagenode", "storagenode-updater")
	})

	configFile, err := os.Open(configPath)
	require.NoError(t, err)
	defer configFile.Close()

	// NB: strip empty lines and comments
	configBuf := bytes.Buffer{}
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

	// TODO: require identity file paths
	//certPath := ctx.File("install", "identity.cert")
	//keyPath := ctx.File("install", "identity.key")

	//expectedCertPath := fmt.Sprintf("identity.cert-path: %s", certPath)
	//expectedKeyPath := fmt.Sprintf("identity.key-path: %s", keyPath)
	expectedEmail := fmt.Sprintf("operator.email: %s", email)
	expectedWallet := fmt.Sprintf("operator.wallet: \"%s\"", walletAddr)
	expectedAddr := fmt.Sprintf("contact.external-address: %s", publicAddr)

	configStr := configBuf.String()
	//require.Contains(t, configStr, expectedCertPath)
	//require.Contains(t, configStr, expectedKeyPath)
	require.Contains(t, configStr, expectedEmail)
	require.Contains(t, configStr, expectedWallet)
	require.Contains(t, configStr,expectedAddr)
}

// TODO: use consistent parameter order for `t` and `ctx`
func install(t *testing.T, ctx *testcontext.Context, msi string, args ...string) {
	logPath := ctx.File("install.log")
	args = append(append([]string{
		"/i", msi,
		"/log", logPath,
	}, msiBaseArgs...), args...)

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

func tryUninstall(t *testing.T, ctx *testcontext.Context, msi string) {
	_, err := uninstall(t, ctx, msi).CombinedOutput()
	if err != nil {
		t.Logf("WARN: tried but failed to uninstall from: %s", msi)
	}
}

func requireUninstall(t *testing.T, ctx *testcontext.Context, msi string) {
	logPath := ctx.File("uninstall.log")
	uninstallOut, err := uninstall(t, ctx, msi).CombinedOutput()
	if err != nil {
		uninstallLogData, err := ioutil.ReadFile(logPath)
		if assert.NoError(t, err) {
			t.Logf("MSIExec log:\n============================\n%s", string(uninstallLogData))
		}
		t.Logf("MSIExec output:\n============================\n%s", string(uninstallOut))
	}
}

func uninstall(t *testing.T, ctx *testcontext.Context, msi string) *exec.Cmd {
	args := append([]string{"/uninstall", msi}, msiBaseArgs...)
	return exec.Command("msiexec", args...)
}

//func buildInstaller(ctx *testcontext.Context, t *testing.T, msi string) {
//	t.Helper()
//
//	require.NotEmpty(t, msi)
//
//	buildInstallerOnce.Do(func() {
//		for name, path := range map[string]string{
//			"storagenode":         storagenodeBin,
//			"storagenode-updater": updaterBin,
//		} {
//			require.NotEmpty(t, path)
//
//			downloadBin(ctx, t, name, path)
//
//			_, err := os.Stat(path)
//			require.NoError(t, err)
//		}
//
//		args := []string{
//			filepath.Join(installerDir, "windows.sln"),
//			"/t:Build",
//			"/p:Configuration=Release",
//		}
//		msbuildOut, err := exec.Command("msbuild", args...).CombinedOutput()
//		if !assert.NoError(t, err) {
//			t.Log(string(msbuildOut))
//			t.Fatal(err)
//		}
//	})
//
//	_, err := os.Stat(msi)
//	require.NoError(t, err)
//}

//func downloadBin(ctx *testcontext.Context, t *testing.T, name, dst string) {
//	t.Helper()
//
//	zipPath := ctx.File("archive", name+".exe.zip")
//	url := releaseUrl(name, ".exe")
//
//	downloadArchive(ctx, t, url, zipPath)
//	unpackBinary(ctx, t, zipPath, dst)
//}

func releaseUrl(name, ext string) string {
	urlTemplate := "https://github.com/storj/storj/releases/download/{version}/{service}_{os}_{arch}{ext}.zip"

	url := strings.Replace(urlTemplate, "{version}", downloadVersion, 1)
	url = strings.Replace(url, "{service}", name, 1)
	url = strings.Replace(url, "{os}", runtime.GOOS, 1)
	url = strings.Replace(url, "{arch}", runtime.GOARCH, 1)
	url = strings.Replace(url, "{ext}", ext, 1)
	return url
}

func downloadInstaller(t *testing.T, ctx *testcontext.Context) {
	t.Helper()

	zipPath := ctx.File("archive", "installer.msi.zip")
	url := releaseUrl("storagenode", ".msi")

	downloadArchive(ctx, t, url, zipPath)
	unpackBinary(ctx, t, zipPath, releaseMSIPath)
}

func downloadArchive(ctx *testcontext.Context, t *testing.T, url, dst string) {
	t.Helper()

	resp, err := http.Get(url)
	require.NoError(t, err)
	defer ctx.Check(resp.Body.Close)

	require.Truef(t, resp.StatusCode == http.StatusOK, resp.Status)

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0755)
	require.NoError(t, err)
	defer ctx.Check(dstFile.Close)

	_, err = sync2.Copy(ctx, dstFile, resp.Body)
	require.NoError(t, err)
}

func unpackBinary(ctx *testcontext.Context, t *testing.T, archive, dst string) {
	zipReader, err := zip.OpenReader(archive)
	require.NoError(t, err)
	defer ctx.Check(zipReader.Close)

	require.Len(t, zipReader.File, 1)

	zipedExec, err := zipReader.File[0].Open()
	require.NoError(t, err)
	defer ctx.Check(zipedExec.Close)

	newExec, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0755)
	require.NoError(t, err)
	defer ctx.Check(newExec.Close)

	_, err = sync2.Copy(ctx, newExec, zipedExec)
	require.NoError(t, err)
}

func stopServices(names ...string) (err error) {
	serviceMgr, err := mgr.Connect()
	if err != nil {
		return err
	}

	group := new(errgroup.Group)
	for _, name := range names {
		service, err := serviceMgr.OpenService(name)
		if err != nil {
			return err
		}
		defer func() {
			err = errs.Combine(err, service.Close())
		}()

		_, err = service.Control(svc.Stop)
		if err != nil {
			return err
		}

		group.Go(waitForStop(service))
	}

	if err = group.Wait(); err != nil {
		return err
	}
	return nil
}

func waitForStop(service *mgr.Service) (func() error) {
	return func() error {
		ctx, _ := context.WithTimeout(context.Background(), 30*time.Second)
		for {
			status, err := service.Query()
			if err != nil {
				return err
			}

			if err := ctx.Err(); err != nil {
				return err
			}

			switch status.State {
			case svc.Stopped:
				return nil
			default:
				time.Sleep(500 * time.Millisecond)
			}
		}
	}
}
