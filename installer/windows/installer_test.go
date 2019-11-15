// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package windows

import (
	"archive/zip"
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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	"storj.io/storj/internal/sync2"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/storagenode"
)

var (
	installerDir   string
	storagenodeBin string
	updaterBin     string
	msiPath        string

	// TODO: make this more dynamic and/or use versioncontrol server?
	// (NB: can't use versioncontrol server until updater process is added to response)
	downloadVersion = "v0.25.1"

	buildInstallerOnce = sync.Once{}
)

func TestMain(m *testing.M) {
	installerDir, err := filepath.Abs(filepath.Join("..", "..", "installer", "windows"))
	if err != nil {
		panic(err)
	}

	storagenodeBin = filepath.Join(installerDir, "storagenode.exe")
	updaterBin = filepath.Join(installerDir, "storagenode-updater.exe")

	msiDir := filepath.Join(installerDir, "bin", "Release")
	msiPath = filepath.Join(msiDir, "storagenode.msi")

	status := m.Run()

	err = os.Remove(storagenodeBin)
	if err != nil && !os.IsNotExist(err) {
		log.Printf("unable to cleanup temp storagenode binary at \"%s\": %s", storagenodeBin, err)
	}
	err = os.Remove(updaterBin)
	if err != nil && !os.IsNotExist(err) {
		log.Printf("unable to cleanup temp updater binary at \"%s\": %s", updaterBin, err)
	}

	os.Exit(status)
}

func TestInstaller_Config(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	requireInstaller(ctx, t)

	installLog := ctx.File("install.log")
	installDir := ctx.Dir("install")
	configFile := ctx.File("install", "config.yaml")

	walletAddr := "0x0000000000000000000000000000000000000000"
	email := "user@mail.test"
	publicAddr := "127.0.0.1:0"

	args := []string{
		"/i", msiPath,
		"/passive", "/qb",
		"/norestart",
		"/log", installLog,
		fmt.Sprintf("INSTALLFOLDER=%s", installDir),
		fmt.Sprintf(`STORJ_WALLET="%s"`, walletAddr),
		fmt.Sprintf(`STORJ_EMAIL="%s"`, email),
		fmt.Sprintf(`STORJ_PUBLIC_ADDRESSS="%s"`, publicAddr),
	}
	installOut, err := exec.Command("msiexec", args...).CombinedOutput()
	if !assert.NoError(t, err) {
		installLogData, err := ioutil.ReadFile(installLog)
		if assert.NoError(t, err) {
			t.Logf("MSIExec install.log:\n============================\n%s", string(installLogData))
		}
		t.Logf("MSIExec output:\n============================\n%s", string(installOut))
		t.Fatalf("MSIExec error:\n============================\n%s", err)
	}

	files, err := ioutil.ReadDir(installDir)
	require.NoError(t, err)
	for _, f := range files {
		t.Log(f.Name())
	}

	configData, err := ioutil.ReadFile(configFile)
	require.NoError(t, err)

	var config storagenode.Config
	err = yaml.Unmarshal(configData, config)
	require.NoError(t, err)

	// TODO: assert config values match input props
	t.Logf("config:\n%+v", config)

	// TODO: uninstall
}

func requireInstaller(ctx *testcontext.Context, t *testing.T) {
	t.Helper()

	require.NotEmpty(t, msiPath)

	buildInstallerOnce.Do(func() {
		for name, path := range map[string]string{
			"storagenode":         storagenodeBin,
			"storagenode-updater": updaterBin,
		} {
			require.NotEmpty(t, path)

			downloadBin(ctx, t, name, path)

			_, err := os.Stat(path)
			require.NoError(t, err)
		}

		args := []string{
			filepath.Join(installerDir, "windows.sln"),
			"/t:Build",
			"/p:Configuration=Release",
		}
		msbuildOut, err := exec.Command("msbuild", args...).CombinedOutput()
		if !assert.NoError(t, err) {
			t.Log(string(msbuildOut))
			t.Fatal(err)
		}
	})

	_, err := os.Stat(msiPath)
	require.NoError(t, err)
}

func downloadBin(ctx *testcontext.Context, t *testing.T, name, dst string) {
	t.Helper()

	zip := ctx.File("archive", name+".exe.zip")
	urlTemplate := "https://github.com/storj/storj/releases/download/{version}/{service}_{os}_{arch}.exe.zip"

	url := strings.Replace(urlTemplate, "{version}", downloadVersion, 1)
	url = strings.Replace(url, "{service}", name, 1)
	url = strings.Replace(url, "{os}", runtime.GOOS, 1)
	url = strings.Replace(url, "{arch}", runtime.GOARCH, 1)

	downloadArchive(ctx, t, url, zip)
	unpackBinary(ctx, t, zip, dst)
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
